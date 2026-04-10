(function () {
  'use strict';
  
  function parseEvent(data) {
    try {
      return JSON.parse(data);
    } catch (e) {
      console.error('[SenSimul] Failed to parse SSE data:', e);
      return null;
    }
  }

  function connectLiveStream(url) {
    console.log('[SenSimul] Connecting to live stream:', url);
    const es = new EventSource(url);
    let reconnectTimer = null;
    
    es.onopen = function() {
      console.log('[SenSimul] Live stream connected');
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
    };
    
    es.onmessage = function (evt) {
      const payload = parseEvent(evt.data);
      if (!payload || payload.kind !== 'live' || !payload.reading) {
        return;
      }
      
      const reading = payload.reading;
      console.log('[SenSimul] Update:', reading.sensor_id, '=', reading.value.toFixed(3));
      
      const row = document.querySelector('tr[data-sensor-id="' + reading.sensor_id + '"]');
      if (!row) {
        console.log('[SenSimul] Row not found for sensor:', reading.sensor_id);
        return;
      }
      
      const valueNode = row.querySelector('.value');
      const statusNode = row.querySelector('.status');
      const updatedNode = row.querySelector('.updated');
      
      if (valueNode) {
        const oldValue = valueNode.textContent;
        const newValue = Number(reading.value).toFixed(3);
        valueNode.textContent = newValue;
        
        if (oldValue !== newValue) {
          valueNode.style.backgroundColor = '#93c5fd';
          valueNode.style.transition = 'background-color 0.3s';
          setTimeout(() => { 
            valueNode.style.backgroundColor = ''; 
          }, 500);
        }
      }
      
      if (statusNode) {
        statusNode.textContent = reading.status;
        if (reading.status === 'stale') {
          statusNode.style.color = '#dc2626';
        } else {
          statusNode.style.color = '';
        }
      }
      
      if (updatedNode) {
        const date = new Date(reading.last_updated);
        updatedNode.textContent = date.toLocaleTimeString();
      }
    };
    
    es.onerror = function(err) {
      console.error('[SenSimul] Live stream error:', err);
      es.close();
      
      if (!reconnectTimer) {
        reconnectTimer = setTimeout(() => {
          console.log('[SenSimul] Reconnecting...');
          connectLiveStream(url);
        }, 3000);
      }
    };
    
    window.addEventListener('beforeunload', function() {
      es.close();
    });
  }
  
  window.sensimulLiveStream = connectLiveStream;
    
    es.onmessage = function (evt) {
      const payload = parseEvent(evt.data);
      if (!payload || payload.kind !== 'live' || !payload.reading) {
        console.warn('Invalid payload received:', evt.data);
        return;
      }
      
      const reading = payload.reading;
      console.log('Received reading:', reading.sensor_id, reading.value);
      
      const row = document.querySelector('tr[data-sensor-id="' + reading.sensor_id + '"]');
      if (!row) {
        console.log('Row not found for sensor:', reading.sensor_id);
        return;
      }
      
      const valueNode = row.querySelector('.value');
      const statusNode = row.querySelector('.status');
      const updatedNode = row.querySelector('.updated');
      
      if (valueNode) {
        valueNode.textContent = Number(reading.value).toFixed(3);
        valueNode.style.backgroundColor = '#e0f2fe';
        setTimeout(() => { valueNode.style.backgroundColor = ''; }, 300);
      }
      if (statusNode) statusNode.textContent = reading.status;
      if (updatedNode) updatedNode.textContent = new Date(reading.last_updated).toLocaleString();
    };
    
    es.onerror = function(err) {
      console.error('Live stream error:', err);
      es.close();
      setTimeout(() => {
        console.log('Reconnecting to live stream...');
        connectLiveStream(url);
      }, 3000);
    };
  }

  function drawSimpleLine(svg, points) {
    if (!svg) return;
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
    console.log('Initializing sensor detail for:', opts.sensorId);
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
      
      if (current) {
        current.textContent = Number(reading.value).toFixed(3);
        current.style.color = '#2563eb';
        setTimeout(() => { current.style.color = ''; }, 300);
      }
      if (status) status.textContent = reading.status;
      drawSimpleLine(chart, reading.points || []);
    };

    const testEs = new EventSource(opts.testEventUrl);
    testEs.onmessage = function (evt) {
      const payload = parseEvent(evt.data);
      if (!payload || payload.kind !== 'test' || !payload.test) return;
      const result = payload.test;
      if (result.sensor_id !== opts.sensorId) return;
      if (testResult) {
        testResult.textContent = 'Test OK: ' + Number(result.value).toFixed(3) + ' (' + result.responded_at + ')';
      }
    };

    if (testBtn) {
      testBtn.addEventListener('click', function () {
        if (testResult) testResult.textContent = 'Running one-shot test...';
        fetch(opts.testUrl, { method: 'POST' })
          .then((r) => {
            if (!r.ok) throw new Error('request failed');
          })
          .catch((e) => {
            console.error('Test failed:', e);
            if (testResult) testResult.textContent = 'Test request failed';
          });
      });
    }
  };
  
  window.sensimulLiveStream = connectLiveStream;
})();
