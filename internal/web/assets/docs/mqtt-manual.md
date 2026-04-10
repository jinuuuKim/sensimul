SenSimul MQTT Integration Manual
================================

Live telemetry topics
---------------------
- `sensimul/sites/{site_id}/sensors/{sensor_id}`

One-shot sensor test topics
---------------------------
- Request: `sensimul/tests/requests/sites/{site_id}/sensors/{sensor_id}`
- Result:  `sensimul/tests/results/sites/{site_id}/sensors/{sensor_id}`

Operational notes
-----------------
- The web service subscribes to live telemetry and test-result topics.
- One-shot test requests are isolated from normal telemetry topics.
- MQTT broker URL is configured through `mqtt.broker_url` in `config/sensimul.yaml`.
