import { sleep, check } from 'k6';
import http from 'k6/http';

export const options = {
	stages: [
		{ duration: '10s', target: 300 },
		{ duration: '30s', target: 300 },
		{ duration: '10s', target: 0 },
	],
	thresholds: {
		http_req_duration: ['p(99)<1500'], // 99% of requests must complete below 1.5s
		http_req_failed: ["rate<0.01"],
	},
	vus: 10,
	noConnectionReuse: true,
	noVUConnectionReuse: false,
};

export default function () {
	let resp = http.get('http://localhost:8080/', {
		headers: {"X-Forwarded-For": "151.100.0.0"},
	});
	check(resp, {
		"status api 200": (r) => r.status === 200,
	});
}
