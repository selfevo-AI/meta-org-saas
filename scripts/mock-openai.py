import json
from http.server import BaseHTTPRequestHandler, HTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get('Content-Length', '0'))
        request = json.loads(self.rfile.read(length))
        context = json.loads(request['messages'][-1]['content'])
        stage = context.get('stage', 'unknown')
        result = {
            'summary': f'{stage} stage evidence analysis',
            'findings': [{
                'title': 'Lifecycle state',
                'evidence': 'project_overview.lifecycle',
                'impact': 'Supports the next-stage decision',
            }],
            'recommendations': [{
                'title': 'Review next action',
                'rationale': 'Derived from verified lifecycle context',
                'priority': 'high',
            }],
            'risks': [{
                'title': 'Execution variance',
                'probability': 'medium',
                'impact': 'May affect delivery',
                'mitigation': 'Require human review before execution',
            }],
            'proposal': {
                'action': 'Estimate current project cost',
                'tool_name': 'project.estimate_cost',
                'arguments': {'stage': stage},
                'requires_approval': True,
            },
            'confidence': 0.88,
            'evidence_refs': [
                'project_overview.project.status',
                'project_overview.lifecycle.next_action',
            ],
        }
        response = {
            'id': f'mock-{stage}',
            'choices': [{'message': {'content': json.dumps(result)}}],
            'usage': {'prompt_tokens': 120, 'completion_tokens': 90},
        }
        payload = json.dumps(response).encode()
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, *_):
        return


if __name__ == '__main__':
    HTTPServer(('0.0.0.0', 8000), Handler).serve_forever()
