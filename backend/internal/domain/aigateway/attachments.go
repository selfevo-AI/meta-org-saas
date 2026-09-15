package aigateway

import (
	"encoding/base64"
	"fmt"
)

func validateMessageAttachments(messages []Message) error {
	count, size := 0, 0
	for _, message := range messages {
		for _, attachment := range message.Attachments {
			count++
			size += len(attachment.Data)
			if message.Role != "user" || len(attachment.Data) == 0 || len(attachment.Data) > 10*1024*1024 || len(attachment.Name) > 240 {
				return fmt.Errorf("%w: invalid user attachment", ErrValidation)
			}
			switch attachment.MediaType {
			case "image/png", "image/jpeg", "image/webp", "application/pdf":
			default:
				return fmt.Errorf("%w: unsupported attachment media type", ErrValidation)
			}
		}
	}
	if count > 5 || size > 20*1024*1024 {
		return fmt.Errorf("%w: attachments exceed limits", ErrValidation)
	}
	return nil
}

func attachmentDataURL(attachment Attachment) string {
	return "data:" + attachment.MediaType + ";base64," + base64.StdEncoding.EncodeToString(attachment.Data)
}

func openAIContent(message Message) any {
	if len(message.Attachments) == 0 {
		return message.Content
	}
	parts := []map[string]any{}
	if message.Content != "" {
		parts = append(parts, map[string]any{"type": "text", "text": message.Content})
	}
	for _, attachment := range message.Attachments {
		if attachment.MediaType == "application/pdf" {
			parts = append(parts, map[string]any{"type": "file", "file": map[string]any{
				"filename": attachment.Name, "file_data": attachmentDataURL(attachment),
			}})
		} else {
			parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": attachmentDataURL(attachment)}})
		}
	}
	return parts
}

func anthropicAttachments(attachments []Attachment) []map[string]any {
	parts := []map[string]any{}
	for _, attachment := range attachments {
		kind := "image"
		if attachment.MediaType == "application/pdf" {
			kind = "document"
		}
		parts = append(parts, map[string]any{"type": kind, "source": map[string]any{
			"type": "base64", "media_type": attachment.MediaType, "data": base64.StdEncoding.EncodeToString(attachment.Data),
		}})
	}
	return parts
}

func geminiAttachments(attachments []Attachment) []map[string]any {
	parts := []map[string]any{}
	for _, attachment := range attachments {
		parts = append(parts, map[string]any{"inlineData": map[string]any{
			"mimeType": attachment.MediaType, "data": base64.StdEncoding.EncodeToString(attachment.Data),
		}})
	}
	return parts
}
