SenSimul Web User Manual
========================

Overview
--------
This web client is an operator-facing interface for SenSimul.

Main screens:
- Sites: add, edit, delete, and navigate to sensor/controller settings.
- Live: see real-time sensor values and detail charts.
- Manual: usage guidance.
- MQTT: topic and integration notes.

Important runtime notes
-----------------------
- Authentication page is intentionally not included. Authentication is expected at reverse proxy level (nginx).
- Live graphs are real-time only in MVP and do not persist historical data.
- Web service can run independently from simulator; live pages may show stale/disconnected status when simulator or broker is unavailable.
