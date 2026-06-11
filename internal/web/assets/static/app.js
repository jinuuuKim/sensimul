(function () {
  'use strict';

  function parseEvent(data) {
    try {
      return JSON.parse(data);
    } catch (e) {
      console.error('[SenSimul] failed to parse SSE data:', e);
      return null;
    }
  }

  function fmtValue(v) {
    if (v === null || v === undefined || isNaN(v)) return '–';
    return Number(v).toFixed(3);
  }

  function fmtTime(iso) {
    if (!iso) return '–';
    var d = new Date(iso);
    if (isNaN(d.getTime())) return '–';
    return d.toLocaleTimeString();
  }

  function escapeHtml(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  function attrEscape(s) {
    return String(s == null ? '' : s).replace(/["\\]/g, '\\$&');
  }

  // ---------------------------------------------------------------------------
  // Live overview table (auto-reconnecting, adds rows for new sensors).
  // ---------------------------------------------------------------------------
  function connectLiveStream(url) {
    var es = null;
    var reconnectTimer = null;
    var closed = false;

    function open() {
      es = new EventSource(url);
      es.onopen = function () {
        if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null; }
      };
      es.onmessage = function (evt) {
        var ev = parseEvent(evt.data);
        if (!ev || ev.kind !== 'live' || !ev.reading) return;
        upsertRow(ev.reading);
      };
      es.onerror = function () {
        if (es) es.close();
        if (!closed && !reconnectTimer) {
          reconnectTimer = setTimeout(function () { reconnectTimer = null; open(); }, 3000);
        }
      };
    }

    open();
    window.addEventListener('beforeunload', function () { closed = true; if (es) es.close(); });
  }

  function upsertRow(reading) {
    var table = document.getElementById('live-table');
    if (!table) return;
    var tbody = table.querySelector('tbody');
    if (!tbody) return;

    var placeholder = tbody.querySelector('tr[data-placeholder]');
    if (placeholder) placeholder.parentNode.removeChild(placeholder);

    var row = tbody.querySelector('tr[data-sensor-id="' + attrEscape(reading.sensor_id) + '"]');
    if (!row) {
      row = document.createElement('tr');
      row.setAttribute('data-sensor-id', reading.sensor_id);
      row.innerHTML =
        '<td>' + escapeHtml(reading.site_id) + '</td>' +
        '<td><a href="/live/sensors/' + encodeURIComponent(reading.sensor_id) + '">' + escapeHtml(reading.sensor_id) + '</a></td>' +
        '<td>' + escapeHtml(reading.sensor_type) + '</td>' +
        '<td class="value"></td><td class="status"></td><td class="updated"></td>';
      tbody.appendChild(row);
    }

    setCell(row.querySelector('.value'), fmtValue(reading.value), true);

    var statusNode = row.querySelector('.status');
    if (statusNode) {
      statusNode.textContent = reading.status || '';
      statusNode.style.color = reading.status === 'stale' ? '#dc2626' : '';
    }

    var updatedNode = row.querySelector('.updated');
    if (updatedNode) updatedNode.textContent = fmtTime(reading.last_updated);
  }

  function setCell(node, text, flash) {
    if (!node) return;
    var old = node.textContent;
    node.textContent = text;
    if (flash && old !== text) {
      node.style.transition = 'background-color .3s';
      node.style.backgroundColor = '#bfdbfe';
      setTimeout(function () { node.style.backgroundColor = ''; }, 500);
    }
  }

  // ---------------------------------------------------------------------------
  // Sensor detail: live value + chart + one-shot test.
  // ---------------------------------------------------------------------------
  var SVG_NS = 'http://www.w3.org/2000/svg';

  function svgEl(name, attrs) {
    var el = document.createElementNS(SVG_NS, name);
    for (var k in attrs) { el.setAttribute(k, attrs[k]); }
    return el;
  }

  function drawChart(svg, points) {
    if (!svg) return;
    while (svg.firstChild) svg.removeChild(svg.firstChild);

    if (!points || points.length === 0) {
      var note = svgEl('text', { x: 400, y: 112, 'text-anchor': 'middle', fill: '#94a3b8', 'font-size': 14 });
      note.textContent = 'Waiting for data…';
      svg.appendChild(note);
      return;
    }

    var W = 800, H = 220, padL = 52, padR = 14, padT = 14, padB = 22;
    var innerW = W - padL - padR, innerH = H - padT - padB;
    var n = points.length;
    var vals = points.map(function (p) { return p.value; });
    var min = Math.min.apply(null, vals), max = Math.max.apply(null, vals);
    if (min === max) { min -= 1; max += 1; }
    var range = max - min;

    function x(i) { return padL + (n === 1 ? innerW / 2 : (i / (n - 1)) * innerW); }
    function y(v) { return padT + innerH - ((v - min) / range) * innerH; }

    [max, (min + max) / 2, min].forEach(function (gv) {
      var gy = y(gv);
      svg.appendChild(svgEl('line', { x1: padL, x2: W - padR, y1: gy, y2: gy, stroke: '#eef2f7', 'stroke-width': 1 }));
      var lbl = svgEl('text', { x: padL - 6, y: gy + 4, 'text-anchor': 'end', fill: '#94a3b8', 'font-size': 11 });
      lbl.textContent = gv.toFixed(1);
      svg.appendChild(lbl);
    });

    var coords = points.map(function (p, i) { return x(i) + ',' + y(p.value); });
    var baseY = padT + innerH;
    svg.appendChild(svgEl('polygon', {
      points: x(0) + ',' + baseY + ' ' + coords.join(' ') + ' ' + x(n - 1) + ',' + baseY,
      fill: 'rgba(37,99,235,0.08)'
    }));
    svg.appendChild(svgEl('polyline', { points: coords.join(' '), fill: 'none', stroke: '#2563eb', 'stroke-width': 2 }));
    svg.appendChild(svgEl('circle', { cx: x(n - 1), cy: y(points[n - 1].value), r: 3.5, fill: '#2563eb' }));
  }

  function sensorDetail(opts) {
    var current = document.getElementById('live-current');
    var status = document.getElementById('live-status');
    var chart = document.getElementById('sensor-chart');
    var testBtn = document.getElementById('sensor-test-btn');
    var testResult = document.getElementById('sensor-test-result');

    var MAX_POINTS = 120;
    var points = Array.isArray(opts.initialPoints) ? opts.initialPoints.slice(-MAX_POINTS) : [];
    drawChart(chart, points);

    var liveEs = null, liveTimer = null, closed = false;
    function openLive() {
      liveEs = new EventSource(opts.liveEventUrl);
      liveEs.onopen = function () { if (liveTimer) { clearTimeout(liveTimer); liveTimer = null; } };
      liveEs.onmessage = function (evt) {
        var ev = parseEvent(evt.data);
        if (!ev || ev.kind !== 'live' || !ev.reading || ev.reading.sensor_id !== opts.sensorId) return;
        var reading = ev.reading;
        if (current) {
          current.textContent = fmtValue(reading.value);
          current.style.color = '#2563eb';
          setTimeout(function () { current.style.color = ''; }, 300);
        }
        if (status) status.textContent = reading.status || '';
        if (Array.isArray(reading.points) && reading.points.length) {
          points = reading.points.slice(-MAX_POINTS);
        } else {
          points.push({ at: reading.last_updated, value: reading.value });
          if (points.length > MAX_POINTS) points = points.slice(-MAX_POINTS);
        }
        drawChart(chart, points);
      };
      liveEs.onerror = function () {
        if (liveEs) liveEs.close();
        if (!closed && !liveTimer) liveTimer = setTimeout(function () { liveTimer = null; openLive(); }, 3000);
      };
    }
    openLive();

    var testEs = new EventSource(opts.testEventUrl);
    testEs.onmessage = function (evt) {
      var ev = parseEvent(evt.data);
      if (!ev || ev.kind !== 'test' || !ev.test || ev.test.sensor_id !== opts.sensorId) return;
      if (testResult) {
        testResult.textContent = 'Test OK: ' + fmtValue(ev.test.value) +
          (ev.test.unit ? ' ' + ev.test.unit : '') + ' @ ' + fmtTime(ev.test.responded_at);
      }
    };

    if (testBtn) {
      testBtn.addEventListener('click', function () {
        if (testResult) testResult.textContent = 'Running one-shot test…';
        fetch(opts.testUrl, { method: 'POST' })
          .then(function (r) { if (!r.ok) throw new Error('HTTP ' + r.status); })
          .catch(function (e) {
            if (testResult) testResult.textContent = 'Test request failed: ' + e.message;
          });
      });
    }

    window.addEventListener('beforeunload', function () {
      closed = true;
      if (liveEs) liveEs.close();
      if (testEs) testEs.close();
    });
  }

  window.sensimulLiveStream = connectLiveStream;
  window.sensimulSensorDetail = sensorDetail;
})();
