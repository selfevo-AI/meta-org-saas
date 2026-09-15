package erp

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const (
	accountCash                = "1002"
	accountReceivable          = "1122"
	accountInventory           = "1405"
	accountAccruedPurchases    = "2202-GRNI"
	accountPayable             = "2202"
	accountInputTax            = "2221-IN"
	accountOutputTax           = "2221-OUT"
	accountRevenue             = "6001"
	accountCostOfSales         = "6401"
	accountPurchaseExpense     = "6602"
	accountInventoryAdjustment = "6901"
)

func money(value float64) float64 {
	return decimal.NewFromFloat(value).Round(6).InexactFloat64()
}

func moneyAdd(left, right float64) float64 {
	return decimal.NewFromFloat(left).Add(decimal.NewFromFloat(right)).Round(6).InexactFloat64()
}

func moneyProduct(left, right float64) float64 {
	return decimal.NewFromFloat(left).Mul(decimal.NewFromFloat(right)).Round(6).InexactFloat64()
}

func validNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && math.Abs(value) < 1e12
}

func documentCurrency(record *Record) string {
	return strings.ToUpper(strings.TrimSpace(stringValue(record.Data, "DocCur", stringValue(record.Data, "Currency", "CNY"))))
}

func validateCurrency(currency string) error {
	if len(currency) != 3 {
		return fmt.Errorf("%w: currency must be a three-letter code", ErrValidation)
	}
	for _, c := range currency {
		if c < 'A' || c > 'Z' {
			return fmt.Errorf("%w: currency must be a three-letter code", ErrValidation)
		}
	}
	return nil
}

func (s *Service) validatePartner(ctx context.Context, document *Record, supplier bool) error {
	key := stringValue(document.Data, "CardCode", "")
	if key == "" {
		return fmt.Errorf("%w: business partner is required", ErrValidation)
	}
	table, _ := s.table("MCRD")
	partner, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return fmt.Errorf("%w: business partner %s does not exist", ErrValidation, key)
	}
	kind := stringValue(partner.Data, "CardType", "")
	if supplier && kind != "S" || !supplier && kind != "C" {
		return fmt.Errorf("%w: business partner type does not match document", ErrValidation)
	}
	if documentFieldEquals(partner, "FrozenFor", "Y") || documentFieldEquals(partner, "ValidFor", "N") {
		return fmt.Errorf("%w: business partner is inactive", ErrValidation)
	}
	return validateCurrency(documentCurrency(document))
}

func (s *Service) prepareOrder(ctx context.Context, tableCode, key, action string) (*ActionResult, error) {
	table, _ := s.table(tableCode)
	if err := s.ensureEditable(ctx, table, key); err != nil {
		return nil, err
	}
	order, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if err := s.validatePartner(ctx, order, tableCode == "MPOR"); err != nil {
		return nil, err
	}
	child := "POR1"
	if tableCode == "MRDR" {
		child = "RDR1"
	}
	lines, err := s.listChildPayloads(ctx, tableCode, key, child)
	if err != nil {
		return nil, err
	}
	net, tax, err := lineTotals(lines)
	if err != nil {
		return nil, err
	}
	data := map[string]any{"DocTotal": moneyAdd(net, tax), "VatSum": tax, "DocCur": documentCurrency(order)}
	if action == "submit" {
		data["DocStatus"] = "S"
	} else {
		data["Confirmed"] = "Y"
	}
	return s.mergeStatusAction(ctx, tableCode, key, action, data)
}

func lineTotals(lines []map[string]any) (float64, float64, error) {
	if len(lines) == 0 {
		return 0, 0, fmt.Errorf("%w: document requires at least one line", ErrValidation)
	}
	net, tax := 0.0, 0.0
	for _, line := range lines {
		quantity, price := numericValue(line, "Quantity"), numericValue(line, "Price")
		rate := numericValue(line, "TaxRate")
		if _, ok := line["TaxRate"]; !ok {
			rate = numericValue(line, "VatPrcnt")
		}
		if !validNumber(quantity) || quantity <= 0 || !validNumber(price) || price < 0 || !validNumber(rate) || rate < 0 || rate > 100 {
			return 0, 0, fmt.Errorf("%w: quantity must be positive, price nonnegative, and tax rate between 0 and 100", ErrValidation)
		}
		if money(quantity) != quantity || money(price) != price {
			return 0, 0, fmt.Errorf("%w: quantity and price support at most six decimal places", ErrValidation)
		}
		amount := moneyProduct(quantity, price)
		if !validNumber(amount) {
			return 0, 0, fmt.Errorf("%w: line amount is too large", ErrValidation)
		}
		net = moneyAdd(net, amount)
		tax = moneyAdd(tax, moneyProduct(amount, rate/100))
	}
	if !validNumber(net) || !validNumber(tax) {
		return 0, 0, fmt.Errorf("%w: document amount is too large", ErrValidation)
	}
	return net, tax, nil
}

func (s *Service) convertOrderToFulfillment(ctx context.Context, tableCode, key string, input ActionInput) (*ActionResult, error) {
	source, _ := s.table(tableCode)
	order, err := s.repo.GetRecord(ctx, source, key)
	if err != nil {
		return nil, err
	}
	if !documentFieldEquals(order, "WddStatus", "A") {
		return nil, fmt.Errorf("%w: order must be approved", ErrValidation)
	}
	if isClosedDocument(order) {
		return nil, fmt.Errorf("%w: order is closed", ErrConflict)
	}
	if err := s.validatePartner(ctx, order, tableCode == "MPOR"); err != nil {
		return nil, err
	}
	targetCode, sourceChild, targetChild, action, prefix := "MPDN", "POR1", "PDN1", "receive", "GR-"
	if tableCode == "MRDR" {
		targetCode, sourceChild, targetChild, action, prefix = "MDLN", "RDR1", "DLN1", "deliver", "DL-"
	}
	if linked := stringValue(order.Data, "FulfillmentEntry", ""); linked != "" {
		targetTable, _ := s.table(targetCode)
		target, err := s.repo.GetRecord(ctx, targetTable, linked)
		if err != nil {
			return nil, err
		}
		return &ActionResult{TableCode: tableCode, Key: key, Action: action, Status: "created", Record: order, GeneratedRecords: []Record{*target}}, nil
	}
	lines, err := s.listChildPayloads(ctx, tableCode, key, sourceChild)
	if err != nil {
		return nil, err
	}
	net, tax, err := lineTotals(lines)
	if err != nil {
		return nil, err
	}
	targetKey := stringValue(input.Data, "DocumentKey", prefix+key)
	target, err := s.createDocument(ctx, tableCode, key, action, targetCode, targetKey, map[string]any{
		"BaseTable": tableCode, "BaseEntry": key, "CardCode": order.Data["CardCode"], "DocCur": documentCurrency(order),
		"DocTotal": moneyAdd(net, tax), "VatSum": tax, "WddStatus": "W", "DocDate": time.Now().UTC().Format("2006-01-02"),
	})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		line = copyData(line)
		line["BaseEntry"], line["BaseLine"] = key, line["LineNum"]
		if _, err := s.createDocumentLine(ctx, targetCode, targetKey, targetChild, fmt.Sprint(i+1), line); err != nil {
			return nil, err
		}
	}
	order, err = s.repo.UpdateRecord(ctx, source, key, RecordInput{Data: map[string]any{"FulfillmentEntry": targetKey, "FulfillmentTable": targetCode}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: tableCode, Key: key, Action: action, Status: "created", Record: order, GeneratedRecords: []Record{*target}}, nil
}

func (s *Service) approveFulfillment(ctx context.Context, tableCode, key string) (*ActionResult, error) {
	table, _ := s.table(tableCode)
	record, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(record) {
		return nil, fmt.Errorf("%w: document is already posted", ErrConflict)
	}
	if err := s.validatePartner(ctx, record, tableCode == "MPDN"); err != nil {
		return nil, err
	}
	child := "PDN1"
	if tableCode == "MDLN" {
		child = "DLN1"
	}
	lines, err := s.listChildPayloads(ctx, tableCode, key, child)
	if err != nil {
		return nil, err
	}
	if _, _, err := lineTotals(lines); err != nil {
		return nil, err
	}
	return s.mergeStatusAction(ctx, tableCode, key, "approve", map[string]any{"WddStatus": "A"})
}

func (s *Service) postStockFulfillment(ctx context.Context, tableCode, key string) (*ActionResult, error) {
	table, _ := s.table(tableCode)
	document, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(document) {
		return &ActionResult{TableCode: tableCode, Key: key, Action: "post", Status: "posted", Record: document}, nil
	}
	if !documentFieldEquals(document, "WddStatus", "A") {
		return nil, fmt.Errorf("%w: document must be approved before posting", ErrValidation)
	}
	if err := s.validatePartner(ctx, document, tableCode == "MPDN"); err != nil {
		return nil, err
	}
	child, movementCode, movementChild, invoiceCode, invoiceChild, prefix, direction := "PDN1", "MIGN", "IGN1", "MPCH", "PCH1", "AP-", 1.0
	if tableCode == "MDLN" {
		child, movementCode, movementChild, invoiceCode, invoiceChild, prefix, direction = "DLN1", "MIGE", "IGE1", "MINV", "INV1", "INV-", -1
	}
	lines, err := s.listChildPayloads(ctx, tableCode, key, child)
	if err != nil {
		return nil, err
	}
	net, tax, err := lineTotals(lines)
	if err != nil {
		return nil, err
	}
	if err := s.validateFulfillmentSource(ctx, document, tableCode, lines); err != nil {
		return nil, err
	}
	currency := documentCurrency(document)
	stockValue := 0.0
	for _, line := range lines {
		value, err := s.adjustValuedInventory(ctx, stringValue(line, "ItemCode", ""), stringValue(line, "WhsCode", ""), numericValue(line, "Quantity"), direction, numericValue(line, "Price"), currency)
		if err != nil {
			return nil, err
		}
		stockValue = moneyAdd(stockValue, value)
	}
	movementPrefix := "IGN-"
	if direction < 0 {
		movementPrefix = "IGE-"
	}
	base := map[string]any{"BaseTable": tableCode, "BaseEntry": key, "CardCode": document.Data["CardCode"], "DocCur": currency,
		"DocTotal": stockValue, "Posted": "Y", "DocStatus": "C", "DocDate": time.Now().UTC().Format("2006-01-02")}
	movement, err := s.createDocument(ctx, tableCode, key, "post", movementCode, movementPrefix+key, base)
	if err != nil {
		return nil, err
	}
	invoice, err := s.createDocument(ctx, tableCode, key, "post", invoiceCode, prefix+key, map[string]any{
		"BaseTable": tableCode, "BaseEntry": key, "CardCode": document.Data["CardCode"], "DocCur": currency,
		"DocTotal": moneyAdd(net, tax), "VatSum": tax, "PaidToDate": 0, "InventoryValue": stockValue, "WddStatus": "A",
		"DocDate": time.Now().UTC().Format("2006-01-02"),
	})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		if _, err := s.createDocumentLine(ctx, movementCode, movement.Key, movementChild, fmt.Sprint(i+1), line); err != nil {
			return nil, err
		}
		if _, err := s.createDocumentLine(ctx, invoiceCode, invoice.Key, invoiceChild, fmt.Sprint(i+1), line); err != nil {
			return nil, err
		}
	}
	generated := []Record{*movement, *invoice}
	if stockValue > 0 {
		debit, credit := accountInventory, accountAccruedPurchases
		if direction < 0 {
			debit, credit = accountCostOfSales, accountInventory
		}
		journal, err := s.createPostedJournal(ctx, tableCode, key, "JE-STOCK-"+tableCode+"-"+key, currency, []map[string]any{
			journalLine(debit, stockValue, 0), journalLine(credit, 0, stockValue),
		})
		if err != nil {
			return nil, err
		}
		generated = append(generated, *journal)
	}
	document, err = s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{
		"DocStatus": "C", "Posted": "Y", "DocTotal": moneyAdd(net, tax), "VatSum": tax, "InvoiceEntry": invoice.Key, "InventoryValue": stockValue,
	}})
	if err != nil {
		return nil, err
	}
	if baseKey := stringValue(document.Data, "BaseEntry", ""); baseKey != "" {
		orderCode := "MPOR"
		if tableCode == "MDLN" {
			orderCode = "MRDR"
		}
		order, _ := s.table(orderCode)
		if _, err := s.repo.UpdateRecord(ctx, order, baseKey, RecordInput{Data: map[string]any{"DocStatus": "C", "FulfillmentStatus": "posted"}}); err != nil {
			return nil, err
		}
	}
	return &ActionResult{TableCode: tableCode, Key: key, Action: "post", Status: "posted", Record: document, GeneratedRecords: generated, Effects: map[string]any{"inventory_value": stockValue}}, nil
}

func (s *Service) validateFulfillmentSource(ctx context.Context, document *Record, tableCode string, lines []map[string]any) error {
	baseKey := stringValue(document.Data, "BaseEntry", "")
	if baseKey == "" {
		return nil
	}
	orderCode, child := "MPOR", "POR1"
	if tableCode == "MDLN" {
		orderCode, child = "MRDR", "RDR1"
	}
	if source := stringValue(document.Data, "BaseTable", orderCode); source != orderCode {
		return fmt.Errorf("%w: invalid fulfillment source type", ErrValidation)
	}
	table, _ := s.table(orderCode)
	order, err := s.repo.GetRecord(ctx, table, baseKey)
	if err != nil {
		return err
	}
	if !documentFieldEquals(order, "WddStatus", "A") || isClosedDocument(order) {
		return fmt.Errorf("%w: source order must be approved and open", ErrValidation)
	}
	if stringValue(order.Data, "CardCode", "") != stringValue(document.Data, "CardCode", "") || documentCurrency(order) != documentCurrency(document) {
		return fmt.Errorf("%w: fulfillment partner and currency must match the order", ErrValidation)
	}
	if linked := stringValue(order.Data, "FulfillmentEntry", ""); linked != "" && linked != document.Key {
		return fmt.Errorf("%w: order already has a fulfillment document", ErrConflict)
	}
	orderLines, err := s.listChildPayloads(ctx, orderCode, baseKey, child)
	if err != nil {
		return err
	}
	if _, _, err := lineTotals(orderLines); err != nil {
		return err
	}
	type lineSignature struct {
		item, warehouse string
		price, tax      float64
	}
	signature := func(line map[string]any) lineSignature {
		rate := numericValue(line, "TaxRate")
		if _, ok := line["TaxRate"]; !ok {
			rate = numericValue(line, "VatPrcnt")
		}
		return lineSignature{stringValue(line, "ItemCode", ""), stringValue(line, "WhsCode", ""), numericValue(line, "Price"), rate}
	}
	quantities := map[lineSignature]float64{}
	for _, line := range orderLines {
		identity := signature(line)
		quantities[identity] = moneyAdd(quantities[identity], numericValue(line, "Quantity"))
	}
	for _, line := range lines {
		identity := signature(line)
		quantities[identity] = moneyAdd(quantities[identity], -numericValue(line, "Quantity"))
		if quantities[identity] < 0 {
			return fmt.Errorf("%w: fulfillment quantities, warehouse, prices and tax must match the order", ErrValidation)
		}
	}
	// The conversion action fulfills one complete order. Partial financial
	// settlement is supported independently by payment allocations.
	for _, remaining := range quantities {
		if remaining != 0 {
			return fmt.Errorf("%w: fulfillment must include the complete order", ErrValidation)
		}
	}
	return nil
}

func (s *Service) adjustValuedInventory(ctx context.Context, itemCode, whsCode string, quantity, direction, unitCost float64, currency string) (float64, error) {
	if itemCode == "" || whsCode == "" || !validNumber(quantity) || quantity <= 0 || !validNumber(unitCost) || unitCost < 0 {
		return 0, fmt.Errorf("%w: item, warehouse, positive quantity, and nonnegative cost are required", ErrValidation)
	}
	if err := validateCurrency(currency); err != nil {
		return 0, err
	}
	if money(quantity) != quantity || (direction != 1 && direction != -1) {
		return 0, fmt.Errorf("%w: invalid inventory quantity or direction", ErrValidation)
	}
	for code, key := range map[string]string{"MITM": itemCode, "MWHS": whsCode} {
		table, _ := s.table(code)
		if _, err := s.repo.GetRecord(ctx, table, key); err != nil {
			return 0, fmt.Errorf("%w: unknown %s %s", ErrValidation, code, key)
		}
	}
	table, _ := s.table("MITW")
	key := itemCode + "|" + whsCode
	current, err := s.repo.GetRecord(ctx, table, key)
	if err != nil && err != ErrNotFound {
		return 0, err
	}
	onHand, value := 0.0, 0.0
	if current != nil {
		onHand, value = numericValue(current.Data, "OnHand"), numericValue(current.Data, "InventoryValue")
		if _, exists := current.Data["InventoryValue"]; !exists {
			average := numericValue(current.Data, "AvgPrice")
			if !validNumber(onHand) || !validNumber(average) {
				return 0, fmt.Errorf("%w: invalid inventory balance", ErrValidation)
			}
			value = moneyProduct(onHand, average)
		}
		if previous := stringValue(current.Data, "Currency", currency); previous != currency && onHand > 0 {
			return 0, fmt.Errorf("%w: inventory currency mismatch", ErrValidation)
		}
	}
	if !validNumber(onHand) || !validNumber(value) || onHand < 0 || value < 0 {
		return 0, fmt.Errorf("%w: invalid inventory balance", ErrValidation)
	}
	next := moneyAdd(onHand, quantity*direction)
	if next < 0 {
		return 0, fmt.Errorf("%w: insufficient inventory for %s/%s", ErrValidation, itemCode, whsCode)
	}
	amount := moneyProduct(quantity, unitCost)
	if direction < 0 {
		amount = 0
		if onHand > 0 {
			amount = decimal.NewFromFloat(value).Mul(decimal.NewFromFloat(quantity)).Div(decimal.NewFromFloat(onHand)).Round(6).InexactFloat64()
		}
		if next == 0 {
			amount = value
		}
	}
	nextValue := moneyAdd(value, amount*direction)
	if !validNumber(next) || !validNumber(nextValue) || !validNumber(amount) || nextValue < 0 {
		return 0, fmt.Errorf("%w: inventory value exceeds limits", ErrValidation)
	}
	avg := 0.0
	if next > 0 {
		avg = decimal.NewFromFloat(nextValue).Div(decimal.NewFromFloat(next)).Round(6).InexactFloat64()
	}
	data := map[string]any{"ItemCode": key, "BaseItemCode": itemCode, "WhsCode": whsCode, "OnHand": next, "InventoryValue": nextValue, "AvgPrice": avg, "Currency": currency}
	if current == nil {
		_, err = s.repo.CreateRecord(ctx, table, RecordInput{Key: key, Data: data})
	} else {
		_, err = s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: data})
	}
	return amount, err
}

func journalLine(account string, debit, credit float64) map[string]any {
	return map[string]any{"AccountCode": account, "Debit": debit, "Credit": credit}
}

func (s *Service) postStandaloneInventory(ctx context.Context, tableCode, key string, direction float64) (*ActionResult, error) {
	table, _ := s.table(tableCode)
	document, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(document) {
		return &ActionResult{TableCode: tableCode, Key: key, Action: "post", Status: "posted", Record: document}, nil
	}
	if stringValue(document.Data, "BaseEntry", "") != "" {
		return nil, fmt.Errorf("%w: generated stock documents must be posted by their source action", ErrConflict)
	}
	child := "IGN1"
	if direction < 0 {
		child = "IGE1"
	}
	lines, err := s.listChildPayloads(ctx, tableCode, key, child)
	if err != nil {
		return nil, err
	}
	if _, _, err := lineTotals(lines); err != nil {
		return nil, err
	}
	value := 0.0
	for _, line := range lines {
		amount, err := s.adjustValuedInventory(ctx, stringValue(line, "ItemCode", ""), stringValue(line, "WhsCode", ""), numericValue(line, "Quantity"), direction, numericValue(line, "Price"), documentCurrency(document))
		if err != nil {
			return nil, err
		}
		value = moneyAdd(value, amount)
	}
	generated := []Record{}
	if value > 0 {
		debit, credit := accountInventory, accountInventoryAdjustment
		if direction < 0 {
			debit, credit = accountInventoryAdjustment, accountInventory
		}
		journal, err := s.createPostedJournal(ctx, tableCode, key, "JE-"+tableCode+"-"+key, documentCurrency(document), []map[string]any{journalLine(debit, value, 0), journalLine(credit, 0, value)})
		if err != nil {
			return nil, err
		}
		generated = append(generated, *journal)
	}
	document, err = s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"DocStatus": "C", "Posted": "Y", "InventoryValue": value}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: tableCode, Key: key, Action: "post", Status: "posted", Record: document, GeneratedRecords: generated}, nil
}

func (s *Service) createPostedJournal(ctx context.Context, sourceTable, sourceKey, journalKey, currency string, lines []map[string]any) (*Record, error) {
	journal, err := s.createDocument(ctx, sourceTable, sourceKey, "post", "MJDT", journalKey, map[string]any{
		"BaseTable": sourceTable, "BaseEntry": sourceKey, "Currency": currency, "Memo": sourceTable + " " + sourceKey,
		"RefDate": time.Now().UTC().Format("2006-01-02"), "BtfStatus": "O",
	})
	if err != nil {
		return nil, err
	}
	for i, line := range lines {
		if _, err := s.createDocumentLine(ctx, "MJDT", journal.Key, "JDT1", fmt.Sprint(i+1), line); err != nil {
			return nil, err
		}
	}
	result, err := s.postJournal(ctx, journal.Key)
	if err != nil {
		return nil, err
	}
	return result.Record, nil
}

func (s *Service) postJournal(ctx context.Context, key string) (*ActionResult, error) {
	table, _ := s.table("MJDT")
	journal, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if documentFieldEquals(journal, "BtfStatus", "V") {
		return nil, fmt.Errorf("%w: void journals cannot be posted", ErrConflict)
	}
	if isPostedDocument(journal) || documentFieldEquals(journal, "BtfStatus", "P") {
		return &ActionResult{TableCode: "MJDT", Key: key, Action: "post", Status: "posted", Record: journal}, nil
	}
	if err := validateCurrency(documentCurrency(journal)); err != nil {
		return nil, err
	}
	lines, err := s.listChildPayloads(ctx, "MJDT", key, "JDT1")
	if err != nil {
		return nil, err
	}
	if len(lines) < 2 {
		return nil, fmt.Errorf("%w: journal requires at least two lines", ErrValidation)
	}
	debit, credit := decimal.Zero, decimal.Zero
	for _, line := range lines {
		d, c := numericValue(line, "Debit"), numericValue(line, "Credit")
		if !validNumber(d) || !validNumber(c) || d < 0 || c < 0 || (d == 0) == (c == 0) {
			return nil, fmt.Errorf("%w: each journal line must have one positive debit or credit", ErrValidation)
		}
		if d != money(d) || c != money(c) {
			return nil, fmt.Errorf("%w: journal amounts support at most six decimal places", ErrValidation)
		}
		accountCode := stringValue(line, "AccountCode", stringValue(line, "Account", ""))
		accountTable, _ := s.table("MACT")
		account, err := s.repo.GetRecord(ctx, accountTable, accountCode)
		if err != nil {
			return nil, fmt.Errorf("%w: journal account %s does not exist", ErrValidation, accountCode)
		}
		if documentFieldEquals(account, "Active", "N") || documentFieldEquals(account, "Active", "false") ||
			documentFieldEquals(account, "Postable", "N") || documentFieldEquals(account, "Postable", "false") {
			return nil, fmt.Errorf("%w: journal account %s is not postable", ErrValidation, accountCode)
		}
		accountCurrency := stringValue(account.Data, "Currency", "")
		if accountCurrency != "" && accountCurrency != "##" && accountCurrency != documentCurrency(journal) {
			return nil, fmt.Errorf("%w: journal account currency mismatch", ErrValidation)
		}
		debit, credit = debit.Add(decimal.NewFromFloat(d).Round(6)), credit.Add(decimal.NewFromFloat(c).Round(6))
	}
	if !debit.Equal(credit) || debit.IsZero() {
		return nil, fmt.Errorf("%w: journal entry is not balanced", ErrValidation)
	}
	if date := stringValue(journal.Data, "RefDate", ""); date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return nil, fmt.Errorf("%w: invalid journal date", ErrValidation)
		}
	}
	return s.mergeStatusAction(ctx, "MJDT", key, "post", map[string]any{"BtfStatus": "P", "Posted": "Y", "PostedAt": time.Now().UTC().Format(time.RFC3339), "RefDate": stringValue(journal.Data, "RefDate", time.Now().UTC().Format("2006-01-02"))})
}

func (s *Service) postFinancialInvoice(ctx context.Context, tableCode, key string) (*ActionResult, error) {
	table, _ := s.table(tableCode)
	invoice, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	if isPostedDocument(invoice) {
		return &ActionResult{TableCode: tableCode, Key: key, Action: "post", Status: "posted", Record: invoice}, nil
	}
	if err := s.validatePartner(ctx, invoice, tableCode == "MPCH"); err != nil {
		return nil, err
	}
	total, tax := numericValue(invoice.Data, "DocTotal"), numericValue(invoice.Data, "VatSum")
	if !validNumber(total) || !validNumber(tax) {
		return nil, fmt.Errorf("%w: invalid invoice amounts", ErrValidation)
	}
	child := "INV1"
	if tableCode == "MPCH" {
		child = "PCH1"
	}
	lines, err := s.listChildPayloads(ctx, tableCode, key, child)
	if err != nil {
		return nil, err
	}
	if len(lines) > 0 {
		net, calculatedTax, err := lineTotals(lines)
		if err != nil {
			return nil, err
		}
		calculatedTotal := moneyAdd(net, calculatedTax)
		if total > 0 && money(total) != calculatedTotal {
			return nil, fmt.Errorf("%w: invoice total does not match its lines", ErrValidation)
		}
		total, tax = calculatedTotal, calculatedTax
	}
	if !validNumber(total) || !validNumber(tax) || total <= 0 || tax < 0 || tax > total {
		return nil, fmt.Errorf("%w: invoice total and tax are invalid", ErrValidation)
	}
	if baseKey := stringValue(invoice.Data, "BaseEntry", ""); baseKey != "" {
		sourceCode := "MDLN"
		if tableCode == "MPCH" {
			sourceCode = "MPDN"
		}
		baseTable, _ := s.table(sourceCode)
		base, err := s.repo.GetRecord(ctx, baseTable, baseKey)
		if err != nil {
			return nil, err
		}
		if !isPostedDocument(base) {
			return nil, fmt.Errorf("%w: source document must be posted", ErrValidation)
		}
		if stringValue(invoice.Data, "BaseTable", sourceCode) != sourceCode || documentCurrency(base) != documentCurrency(invoice) ||
			stringValue(base.Data, "CardCode", "") != stringValue(invoice.Data, "CardCode", "") || money(total) != money(numericValue(base.Data, "DocTotal")) || money(tax) != money(numericValue(base.Data, "VatSum")) {
			return nil, fmt.Errorf("%w: invoice must match its source document", ErrValidation)
		}
		if stringValue(base.Data, "InvoiceEntry", key) != key {
			return nil, fmt.Errorf("%w: source document already has an invoice", ErrConflict)
		}
	}
	net := moneyAdd(total, -tax)
	journalLines := []map[string]any{journalLine(accountReceivable, total, 0)}
	if net > 0 {
		journalLines = append(journalLines, journalLine(accountRevenue, 0, net))
	}
	if tax > 0 {
		journalLines = append(journalLines, journalLine(accountOutputTax, 0, tax))
	}
	if tableCode == "MPCH" {
		debitAccount := accountPurchaseExpense
		if stringValue(invoice.Data, "BaseEntry", "") != "" {
			debitAccount = accountAccruedPurchases
		}
		journalLines = []map[string]any{journalLine(accountPayable, 0, total)}
		if net > 0 {
			journalLines = append(journalLines, journalLine(debitAccount, net, 0))
		}
		if tax > 0 {
			journalLines = append(journalLines, journalLine(accountInputTax, tax, 0))
		}
	}
	journal, err := s.createPostedJournal(ctx, tableCode, key, "JE-"+tableCode+"-"+key, documentCurrency(invoice), journalLines)
	if err != nil {
		return nil, err
	}
	invoice, err = s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"Posted": "Y", "DocTotal": total, "VatSum": tax, "JournalEntry": journal.Key}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: tableCode, Key: key, Action: "post", Status: "posted", Record: invoice, GeneratedRecords: []Record{*journal}}, nil
}

func (s *Service) allocatePayment(ctx context.Context, tableCode, key string, input ActionInput) (*ActionResult, error) {
	targetCode, child := "MINV", "RCT1"
	if tableCode == "MVPM" {
		targetCode, child = "MPCH", "VPM1"
	}
	if requested := stringValue(input.Data, "TargetTable", targetCode); requested != targetCode {
		return nil, fmt.Errorf("%w: payment target must be %s", ErrValidation, targetCode)
	}
	targetKey, amount := stringValue(input.Data, "TargetKey", ""), numericValue(input.Data, "Amount")
	if targetKey == "" || !validNumber(amount) || amount <= 0 {
		return nil, fmt.Errorf("%w: target key and positive amount are required", ErrValidation)
	}
	amount = money(amount)
	if amount == 0 {
		return nil, fmt.Errorf("%w: allocation amount is below ledger precision", ErrValidation)
	}
	table, _ := s.table(tableCode)
	payment, err := s.repo.GetRecord(ctx, table, key)
	if err != nil {
		return nil, err
	}
	targetTable, _ := s.table(targetCode)
	invoice, err := s.repo.GetRecord(ctx, targetTable, targetKey)
	if err != nil {
		return nil, err
	}
	if !isPostedDocument(invoice) {
		return nil, fmt.Errorf("%w: invoice must be posted before settlement", ErrValidation)
	}
	if documentCurrency(payment) != documentCurrency(invoice) {
		return nil, fmt.Errorf("%w: payment and invoice currency mismatch", ErrValidation)
	}
	partner := stringValue(payment.Data, "CardCode", "")
	if partner != "" && partner != stringValue(invoice.Data, "CardCode", "") {
		return nil, fmt.Errorf("%w: payment and invoice partner mismatch", ErrValidation)
	}
	if err := s.validatePartner(ctx, invoice, tableCode == "MVPM"); err != nil {
		return nil, err
	}
	total, allocated := numericValue(payment.Data, "DocTotal"), numericValue(payment.Data, "AllocatedAmount")
	if !validNumber(total) || !validNumber(allocated) || !validNumber(numericValue(payment.Data, "OpenBal")) {
		return nil, fmt.Errorf("%w: invalid payment balance", ErrValidation)
	}
	if _, exists := payment.Data["AllocatedAmount"]; !exists {
		if _, hasBalance := payment.Data["OpenBal"]; hasBalance {
			allocated = moneyAdd(total, -numericValue(payment.Data, "OpenBal"))
		}
	}
	invoiceTotal, paid := numericValue(invoice.Data, "DocTotal"), numericValue(invoice.Data, "PaidToDate")
	if !validNumber(total) || !validNumber(allocated) || !validNumber(invoiceTotal) || !validNumber(paid) || total <= 0 || allocated < 0 || paid < 0 {
		return nil, fmt.Errorf("%w: invalid payment or invoice balance", ErrValidation)
	}
	openPayment, openInvoice := moneyAdd(total, -allocated), moneyAdd(invoiceTotal, -paid)
	if amount > openPayment || amount > openInvoice {
		return nil, fmt.Errorf("%w: allocation exceeds payment or invoice open balance", ErrValidation)
	}
	debit, credit := accountCash, accountReceivable
	if tableCode == "MVPM" {
		debit, credit = accountPayable, accountCash
	}
	meta := actionExecutionFromContext(ctx)
	journalKey := "JE-" + tableCode + "-" + key + "-" + meta.ExecutionID.String()
	journal, err := s.createPostedJournal(ctx, tableCode, key, journalKey, documentCurrency(payment), []map[string]any{journalLine(debit, amount, 0), journalLine(credit, 0, amount)})
	if err != nil {
		return nil, err
	}
	allocations, err := s.listChildPayloads(ctx, tableCode, key, child)
	if err != nil {
		return nil, err
	}
	lineNumber := 1
	for _, allocation := range allocations {
		number, err := strconv.Atoi(stringValue(allocation, "LineNum", "0"))
		if err != nil || number >= 2147483647 {
			return nil, fmt.Errorf("%w: invalid allocation line number", ErrValidation)
		}
		if number >= lineNumber {
			lineNumber = number + 1
		}
	}
	if _, err := s.createDocumentLine(ctx, tableCode, key, child, fmt.Sprint(lineNumber), map[string]any{"TargetTable": targetCode, "TargetKey": targetKey, "Amount": amount, "JournalEntry": journalKey, "ActionID": meta.ExecutionID.String()}); err != nil {
		return nil, err
	}
	paid, allocated = moneyAdd(paid, amount), moneyAdd(allocated, amount)
	status := "O"
	if paid == money(invoiceTotal) {
		status = "C"
	}
	if _, err := s.repo.UpdateRecord(ctx, targetTable, targetKey, RecordInput{Data: map[string]any{"PaidToDate": paid, "DocStatus": status}}); err != nil {
		return nil, err
	}
	status = "O"
	if allocated == money(total) {
		status = "C"
	}
	payment, err = s.repo.UpdateRecord(ctx, table, key, RecordInput{Data: map[string]any{"OpenBal": moneyAdd(total, -allocated), "AllocatedAmount": allocated, "DocStatus": status, "CardCode": invoice.Data["CardCode"]}})
	if err != nil {
		return nil, err
	}
	return &ActionResult{TableCode: tableCode, Key: key, Action: "allocate", Status: "allocated", Record: payment, GeneratedRecords: []Record{*journal}, Effects: map[string]any{"allocated_amount": amount, "target_table": targetCode, "target_key": targetKey, "invoice_open_balance": moneyAdd(invoiceTotal, -paid)}}, nil
}
