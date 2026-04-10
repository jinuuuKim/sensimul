// Module: Live stream UI wiring for overview, detail charts, and one-shot tests.
(function () {
  function parseEvent(data) {
    try {
      return JSON.parse(data);
    } catch (_) {
      return null;
    }
  }

  window.sensimulLiveStream = function (url) {
    const es = new EventSource(url);
    es.onmessage = function (evt) {
      const payload = parseEvent(evt.data);
      if (!payload || payload.kind !== 'live' || !payload.reading) return;
      const reading = payload.reading;
      const row = document.querySelector('tr[data-sensor-id="' + reading.sensor_id + '"]');
      if (!row) return;
      const valueNode = row.querySelector('.value');
      const statusNode = row.querySelector('.status');
      const updatedNode = row.querySelector('.updated');
      if (valueNode) valueNode.textContent = Number(reading.value).toFixed(3);
      if (statusNode) statusNode.textContent = reading.status;
      if (updatedNode) updatedNode.textContent = reading.last_updated;
    };
  };

  function drawSimpleLine(svg, points) {
    while (svg.firstChild) svg.removeChild(svg.firstChild);
    if (!points || points.length < 2) return;

    const w = 800;
    const h = 220;
    const min = Math.min.apply(null, points.map(p => p.value));
    const max = Math.max.apply(null, points.map(p => p.value));
    const range = (max - min) || 1;
    const step = w / Math.max(1, points.length - 1);

    const poly = document.createElementNS('http://www.w3.org/2000/svg', 'polyline');
    const coords = points.map((p, i) => {
      const x = i * step;
      const y = h - ((p.value - min) / range) * (h - 20) - 10;
      return `${x},${y}`;
    }).join(' ');

    poly.setAttribute('points', coords);
    poly.setAttribute('fill', 'none');
    poly.setAttribute('stroke', '#2563eb');
    poly.setAttribute('stroke-width', '2');
    svg.appendChild(poly);
  }

  window.sensimulSensorDetail = function (opts) {
    const current = document.getElementById('live-current');
    const status = document.getElementById('live-status');
    const chart = document.getElementById('sensor-chart');
    const testBtn = document.getElementById('sensor-test-btn');
    const testResult = document.getElementById('sensor-test-result');

    const liveEs = new EventSource(opts.liveEventUrl);
    liveEs.onmessage = function (evt) {
      const payload = parseEvent(evt.data);
      if (!payload || payload.kind !== 'live' || !payload.reading) return;
      const reading = payload.reading;
      if (reading.sensor_id !== opts.sensorId) return;
      current.textContent = Number(reading.value).toFixed(3);
      status.textContent = reading.status;
      drawSimpleLine(chart, reading.points || []);
    };

    const testEs = new EventSource(opts.testEventUrl);
    testEs.onmessage = function (evt) {
      const payload = parseEvent(evt.data);
      if (!payload || payload.kind !== 'test' || !payload.test) return;
      const result = payload.test;
      if (result.sensor_id !== opts.sensorId) return;
      testResult.textContent = `Test OK: ${Number(result.value).toFixed(3)} (${result.responded_at})`;
    };

    if (testBtn) {
      testBtn.addEventListener('click', function () {
        testResult.textContent = 'Running one-shot test...';
        fetch(opts.testUrl, { method: 'POST' })
          .then((r) => {
            if (!r.ok) throw new Error('request failed');
          })
          .catch(() => {
            testResult.textContent = 'Test request failed';
          });
      });
    }
  };
})();
