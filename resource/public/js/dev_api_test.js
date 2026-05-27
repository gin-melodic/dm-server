// API Presets extracted from api.json (OpenAPI 3.0)
const API_PRESETS = [
  // --- 用户管理 ---
  {
    group: '用户管理',
    endpoints: [
      {
        label: 'GET /api/v1/user/info — 获取用户信息',
        method: 'GET',
        url: '/api/v1/user/info',
        params: [],
        headers: [],
        body: ''
      },
      {
        label: 'PUT /api/v1/user/info — 更新用户信息',
        method: 'PUT',
        url: '/api/v1/user/info',
        params: [],
        headers: [],
        body: JSON.stringify({
          "id": 1,
          "openid": "oXXXXXXXXXXXXXXXXXXXXXXX",
          "nickname": "测试用户",
          "avatar_url": "https://example.com/avatar.jpg"
        }, null, 2)
      }
    ]
  },
  // --- 用户认证 ---
  {
    group: '用户认证',
    endpoints: [
      {
        label: 'POST /api/v1/wechat/auth — 微信小程序授权登录',
        method: 'POST',
        url: '/api/v1/wechat/auth',
        params: [],
        headers: [],
        body: JSON.stringify({
          "code": "0a1xYZ2yZ3xYZ4aB5cD6eF7gH8"
        }, null, 2)
      }
    ]
  },
  // --- Dream ---
  {
    group: 'Dream',
    endpoints: [
      {
        label: 'GET /api/v1/dream/list — 获取梦境列表',
        method: 'GET',
        url: '/api/v1/dream/list',
        params: [
          { key: 'startDate', value: '2022-01-01' },
          { key: 'endDate', value: '2022-12-31' },
          { key: 'pageSize', value: '10' },
          { key: 'page', value: '1' }
        ],
        headers: [],
        body: ''
      },
      {
        label: 'GET /api/v1/dream/analyze/result — 获取梦境分析结果',
        method: 'GET',
        url: '/api/v1/dream/analyze/result',
        params: [
          { key: 'id', value: '1' }
        ],
        headers: [],
        body: ''
      },
      {
        label: 'POST /api/v1/dream/delete — 删除梦境',
        method: 'POST',
        url: '/api/v1/dream/delete',
        params: [],
        headers: [],
        body: JSON.stringify({
          "id": 1
        }, null, 2)
      }
    ]
  },
  // --- Chat ---
  {
    group: 'Chat',
    endpoints: [
      {
        label: 'GET /api/v1/chat/ws — WebSocket聊天连接',
        method: 'GET',
        url: '/api/v1/chat/ws',
        params: [],
        headers: [],
        body: ''
      }
    ]
  }
];

document.addEventListener('DOMContentLoaded', () => {
  // DOM Elements
  const apiPresetSelect = document.getElementById('api-preset-select');
  const methodSelect = document.getElementById('method-select');
  const urlInput = document.getElementById('url-input');
  const btnSend = document.getElementById('btn-send');

  const queryParamsContainer = document.getElementById('query-params-container');
  const headersContainer = document.getElementById('headers-container');
  const bodyTextarea = document.getElementById('body-textarea');
  const btnFormatJson = document.getElementById('btn-format-json');

  const responsePlaceholder = document.getElementById('response-placeholder');
  const responseContent = document.getElementById('response-content');
  const responseStatus = document.getElementById('response-status');
  const responseTime = document.getElementById('response-time');
  const responseBodyPre = document.getElementById('response-body-pre');
  const responseHeadersTable = document.getElementById('response-headers-table').querySelector('tbody');

  const historyList = document.getElementById('history-list');
  const btnAddParam = document.getElementById('btn-add-param');
  const btnAddHeader = document.getElementById('btn-add-header');

  const authPresetCheck = document.getElementById('auth-preset-check');
  const authTokenInput = document.getElementById('auth-token-input');

  // WebSocket DOM Elements
  const wsConsole = document.getElementById('ws-console');
  const wsStatusDot = document.getElementById('ws-status-dot');
  const wsStatusText = document.getElementById('ws-status-text');
  const wsMessages = document.getElementById('ws-messages');
  const wsMessageInput = document.getElementById('ws-message-input');
  const btnWsSend = document.getElementById('btn-ws-send');
  const btnWsClose = document.getElementById('btn-ws-close');

  // WebSocket State
  let wsConnection = null;

  // State
  let activeRequest = {
    method: 'GET',
    url: '',
    headers: {},
    body: ''
  };

  // Initialize
  initTabs();
  initKeyValueEditors();
  initAuthPreset();
  initHistory();
  initMethodColors();
  initApiPresets();

  apiPresetSelect.addEventListener('change', loadApiPreset);

  // WebSocket event listeners
  btnWsClose.addEventListener('click', closeWebSocket);
  btnWsSend.addEventListener('click', sendWsMessage);
  wsMessageInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendWsMessage();
    }
  });

  // Format JSON handler
  btnFormatJson.addEventListener('click', () => {
    try {
      const raw = bodyTextarea.value.trim();
      if (!raw) return;
      const parsed = JSON.parse(raw);
      bodyTextarea.value = JSON.stringify(parsed, null, 2);
    } catch (e) {
      alert('Invalid JSON: ' + e.message);
    }
  });

  // Method Color Sync
  methodSelect.addEventListener('change', updateMethodColor);
  function updateMethodColor() {
    const val = methodSelect.value.toLowerCase();
    methodSelect.className = `method-select bg-${val}`;
  }
  function initMethodColors() {
    updateMethodColor();
  }

  // Tabs management
  function initTabs() {
    document.querySelectorAll('.tabs-container').forEach(container => {
      const tabs = container.querySelectorAll('.tab');
      tabs.forEach(tab => {
        tab.addEventListener('click', () => {
          const targetId = tab.dataset.target;
          const pane = document.getElementById(targetId);

          if (!pane) return;

          // Deactivate all in this group
          container.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
          const parent = container.parentElement;
          parent.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));

          // Activate current
          tab.classList.add('active');
          pane.classList.add('active');
        });
      });
    });
  }

  // Dynamic Key-Value Row Builders (Parameters and Headers)
  function initKeyValueEditors() {
    // Add default row
    addKeyValueRow(queryParamsContainer, '', '', syncParamsToUrl);
    addKeyValueRow(headersContainer, '', '', () => { });

    btnAddParam.addEventListener('click', () => {
      addKeyValueRow(queryParamsContainer, '', '', syncParamsToUrl);
    });

    btnAddHeader.addEventListener('click', () => {
      addKeyValueRow(headersContainer, '', '', () => { });
    });

    // Parse URL on input to extract query params
    urlInput.addEventListener('input', parseUrlToParams);
  }

  function addKeyValueRow(container, key = '', value = '', onChange = null) {
    const row = document.createElement('div');
    row.className = 'kv-row';

    const keyInput = document.createElement('input');
    keyInput.type = 'text';
    keyInput.placeholder = 'Key';
    keyInput.className = 'kv-input kv-input-key';
    keyInput.value = key;

    const valueInput = document.createElement('input');
    valueInput.type = 'text';
    valueInput.placeholder = 'Value';
    valueInput.className = 'kv-input kv-input-value';
    valueInput.value = value;

    const deleteBtn = document.createElement('button');
    deleteBtn.className = 'btn-icon';
    deleteBtn.innerHTML = `
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <line x1="18" y1="6" x2="6" y2="18"></line>
        <line x1="6" y1="6" x2="18" y2="18"></line>
      </svg>
    `;

    row.appendChild(keyInput);
    row.appendChild(valueInput);
    row.appendChild(deleteBtn);
    container.appendChild(row);

    const triggerChange = () => {
      if (onChange) onChange();
    };

    keyInput.addEventListener('input', triggerChange);
    valueInput.addEventListener('input', triggerChange);

    deleteBtn.addEventListener('click', () => {
      row.remove();
      // Ensure there is at least one empty row
      if (container.querySelectorAll('.kv-row').length === 0) {
        addKeyValueRow(container, '', '', onChange);
      }
      triggerChange();
    });
  }

  function getKeyValuePairs(container) {
    const pairs = {};
    container.querySelectorAll('.kv-row').forEach(row => {
      const key = row.querySelector('.kv-input-key').value.trim();
      const val = row.querySelector('.kv-input-value').value.trim();
      if (key) {
        pairs[key] = val;
      }
    });
    return pairs;
  }

  // Synchronize Params Editor with URL Bar
  function syncParamsToUrl() {
    try {
      const urlStr = urlInput.value.trim();
      if (!urlStr) return;

      const u = new URL(urlStr.startsWith('http') ? urlStr : `http://${urlStr}`);
      const params = getKeyValuePairs(queryParamsContainer);
      u.search = '';
      Object.keys(params).forEach(k => {
        u.searchParams.set(k, params[k]);
      });

      // Preserve absolute/relative path inputs
      let finalUrl = urlStr.split('?')[0];
      const searchStr = u.search;
      urlInput.value = finalUrl + searchStr;
    } catch (e) {
      // URL parsing failed, skip sync
    }
  }

  function parseUrlToParams() {
    try {
      const urlStr = urlInput.value.trim();
      if (!urlStr || !urlStr.includes('?')) return;

      const search = urlStr.substring(urlStr.indexOf('?'));
      const searchParams = new URLSearchParams(search);

      // Clear container and rebuild
      queryParamsContainer.innerHTML = '';

      let count = 0;
      searchParams.forEach((value, key) => {
        addKeyValueRow(queryParamsContainer, key, value, syncParamsToUrl);
        count++;
      });

      if (count === 0) {
        addKeyValueRow(queryParamsContainer, '', '', syncParamsToUrl);
      }
    } catch (e) {
      // Skip parsing if invalid URL
    }
  }

  // Authorization Cache Panel
  function initAuthPreset() {
    const cachedToken = localStorage.getItem('gf_dev_token');
    if (cachedToken) {
      authTokenInput.value = cachedToken;
    }

    const cachedCheck = localStorage.getItem('gf_dev_auth_enable');
    if (cachedCheck === 'true') {
      authPresetCheck.checked = true;
    }

    authPresetCheck.addEventListener('change', () => {
      localStorage.setItem('gf_dev_auth_enable', authPresetCheck.checked);
    });

    authTokenInput.addEventListener('input', () => {
      localStorage.setItem('gf_dev_token', authTokenInput.value.trim());
    });
  }

  // Save history of requests
  function initHistory() {
    renderHistory();
  }

  function saveRequestToHistory(req) {
    let history = JSON.parse(localStorage.getItem('gf_dev_history') || '[]');
    // Avoid duplicate URLs with the same method
    history = history.filter(item => !(item.url === req.url && item.method === req.method));

    // Add to top
    history.unshift({
      method: req.method,
      url: req.url,
      headers: req.headers,
      body: req.body,
      timestamp: Date.now()
    });

    // Cap at 15 items
    if (history.length > 15) history.pop();

    localStorage.setItem('gf_dev_history', JSON.stringify(history));
    renderHistory();
  }

  function renderHistory() {
    const history = JSON.parse(localStorage.getItem('gf_dev_history') || '[]');
    historyList.innerHTML = '';

    if (history.length === 0) {
      historyList.innerHTML = `<div class="history-empty">No recent requests</div>`;
      return;
    }

    history.forEach(item => {
      const el = document.createElement('div');
      el.className = 'history-item';

      const methodSpan = document.createElement('span');
      methodSpan.className = `history-method method-${item.method.toLowerCase()} bg-${item.method.toLowerCase()}`;
      methodSpan.textContent = item.method;

      const urlSpan = document.createElement('span');
      urlSpan.className = 'history-url';
      urlSpan.textContent = item.url;

      el.appendChild(methodSpan);
      el.appendChild(urlSpan);
      historyList.appendChild(el);

      el.addEventListener('click', () => {
        // Load into form
        methodSelect.value = item.method;
        updateMethodColor();
        urlInput.value = item.url;
        bodyTextarea.value = item.body || '';

        // Rebuild query parameters
        parseUrlToParams();

        // Rebuild headers
        headersContainer.innerHTML = '';
        const headers = item.headers || {};
        let headerCount = 0;
        Object.keys(headers).forEach(k => {
          addKeyValueRow(headersContainer, k, headers[k], () => { });
          headerCount++;
        });
        if (headerCount === 0) {
          addKeyValueRow(headersContainer, '', '', () => { });
        }

        // Show active state
        document.querySelectorAll('.history-item').forEach(hi => hi.classList.remove('active'));
        el.classList.add('active');
      });
    });
  }

  // Send request
  btnSend.addEventListener('click', sendRequest);

  async function sendRequest() {
    let url = urlInput.value.trim();
    if (!url) {
      alert('Please enter a URL');
      return;
    }

    // Detect WebSocket endpoint: URL contains /ws or starts with ws:// or wss://
    const isWebSocket = url.includes('/ws') || url.startsWith('ws://') || url.startsWith('wss://');

    if (isWebSocket) {
      openWebSocket(url);
      return;
    }

    // Auto prepend http if needed
    if (!url.startsWith('http://') && !url.startsWith('https://') && !url.startsWith('/')) {
      // If it doesn't start with a slash or scheme, assume it's a domain/ip on local host port or external
      url = window.location.protocol + '//' + url;
    } else if (url.startsWith('/')) {
      // If it starts with a slash, assume it is relative to current page host
      url = window.location.origin + url;
    }

    const method = methodSelect.value;
    const body = bodyTextarea.value;

    // Headers list
    const headers = getKeyValuePairs(headersContainer);

    // Merge Bearer token if checked; fall back to dev bypass header
    if (authPresetCheck.checked) {
      const token = authTokenInput.value.trim();
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      } else {
        headers['X-Dev-Token'] = 'nowled2_token';
      }
    }

    // Set content type if not set and method is not GET
    if (method !== 'GET' && !headers['Content-Type'] && !headers['content-type']) {
      headers['Content-Type'] = 'application/json';
    }

    // Toggle Send UI
    btnSend.disabled = true;
    btnSend.innerHTML = `<span class="spinner"></span> Executing...`;

    // Close WS if open and switch to HTTP view
    if (wsConnection) {
      wsConnection.close();
      wsConnection = null;
    }
    responsePlaceholder.style.display = 'none';
    responseContent.style.display = 'block';
    wsConsole.style.display = 'none';

    responseStatus.className = 'status-badge';
    responseStatus.textContent = 'PENDING';
    responseTime.textContent = '... ms';
    responseBodyPre.innerHTML = '<span style="color: var(--text-muted)">Sending request to proxy...</span>';
    responseHeadersTable.innerHTML = '';

    const payload = {
      method,
      url,
      headers,
      body: method !== 'GET' ? body : ''
    };

    try {
      const startLocalTime = Date.now();
      const res = await fetch('/dev/api-test/proxy', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(payload)
      });

      const localDuration = Date.now() - startLocalTime;
      const data = await res.json();

      if (data.error) {
        renderError(data.error);
        btnSend.disabled = false;
        btnSend.innerHTML = `
          <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="22" y1="2" x2="11" y2="13"></line>
            <polygon points="22 2 15 22 11 13 2 9 22 2"></polygon>
          </svg> Send
        `;
        return;
      }

      // Save successful request to history
      saveRequestToHistory(payload);

      // Render Status and Timings
      const status = data.status || 200;
      const statusText = data.statusText || 'OK';
      const statusClass = status >= 500 ? 'status-5xx' : status >= 400 ? 'status-4xx' : status >= 300 ? 'status-3xx' : 'status-2xx';

      responseStatus.className = `status-badge ${statusClass}`;
      responseStatus.textContent = `${status} ${statusText}`;

      // Render times (prefer precise backend duration, fallback to local fetch timing)
      responseTime.textContent = `${data.timeMs || localDuration} ms`;

      // Render Headers
      responseHeadersTable.innerHTML = '';
      const respHeaders = data.headers || {};
      Object.keys(respHeaders).forEach(k => {
        const row = document.createElement('tr');
        row.innerHTML = `
          <td>${k}</td>
          <td>${respHeaders[k]}</td>
        `;
        responseHeadersTable.appendChild(row);
      });

      // Render Response Body with syntax highlighting
      const bodyText = data.body || '';
      try {
        const parsedJson = JSON.parse(bodyText);
        responseBodyPre.innerHTML = syntaxHighlightJson(parsedJson);
      } catch (err) {
        // Plain text
        responseBodyPre.textContent = bodyText || '(Empty Response)';
      }

    } catch (err) {
      renderError(err.message);
    } finally {
      btnSend.disabled = false;
      btnSend.innerHTML = `
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <line x1="22" y1="2" x2="11" y2="13"></line>
          <polygon points="22 2 15 22 11 13 2 9 22 2"></polygon>
        </svg> Send
      `;
    }
  }

  function renderError(errMsg) {
    responsePlaceholder.style.display = 'none';
    responseContent.style.display = 'block';
    wsConsole.style.display = 'none';
    responseStatus.className = 'status-badge status-5xx';
    responseStatus.textContent = 'ERROR';
    responseTime.textContent = '0 ms';
    responseBodyPre.innerHTML = `<span style="color: #ef4444; font-weight: 500;">Request execution failed:</span>\n\n${errMsg}`;
  }

  // JSON syntax highlighting helper
  function syntaxHighlightJson(json) {
    if (typeof json !== 'string') {
      json = JSON.stringify(json, null, 2);
    }
    // Escape HTML special characters
    json = json.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    return json.replace(/("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g, function (match) {
      let cls = 'json-value-number';
      if (/^"/.test(match)) {
        if (/:$/.test(match)) {
          cls = 'json-key';
        } else {
          cls = 'json-value-string';
        }
      } else if (/true|false/.test(match)) {
        cls = 'json-value-boolean';
      } else if (/null/.test(match)) {
        cls = 'json-value-null';
      }
      return '<span class="' + cls + '">' + match + '</span>';
    });
  }

  // API Presets: populate dropdown from api.json definitions
  function initApiPresets() {
    apiPresetSelect.innerHTML = '<option value="">-- Select API Endpoint --</option>';

    API_PRESETS.forEach(group => {
      const optgroup = document.createElement('optgroup');
      optgroup.label = group.group;

      group.endpoints.forEach((ep, idx) => {
        const option = document.createElement('option');
        // Build a unique value: "groupIndex:endpointIndex"
        const groupIdx = API_PRESETS.indexOf(group);
        option.value = `${groupIdx}:${idx}`;
        option.textContent = ep.label;
        optgroup.appendChild(option);
      });

      apiPresetSelect.appendChild(optgroup);
    });
  }

  // Load selected preset into the form
  function loadApiPreset() {
    const val = apiPresetSelect.value;
    if (!val) return;

    const [groupIdx, epIdx] = val.split(':').map(Number);
    const preset = API_PRESETS[groupIdx].endpoints[epIdx];

    // Set method
    methodSelect.value = preset.method;
    updateMethodColor();

    // Set URL
    urlInput.value = preset.url;

    // Clear and rebuild query params
    queryParamsContainer.innerHTML = '';
    if (preset.params && preset.params.length > 0) {
      preset.params.forEach(p => {
        addKeyValueRow(queryParamsContainer, p.key, p.value, syncParamsToUrl);
      });
    } else {
      addKeyValueRow(queryParamsContainer, '', '', syncParamsToUrl);
    }

    // Clear and rebuild headers
    headersContainer.innerHTML = '';
    if (preset.headers && preset.headers.length > 0) {
      preset.headers.forEach(h => {
        addKeyValueRow(headersContainer, h.key, h.value, () => { });
      });
    } else {
      addKeyValueRow(headersContainer, '', '', () => { });
    }

    // Set body
    bodyTextarea.value = preset.body || '';

    // Sync params container into URL
    syncParamsToUrl();
  }

  // ===== WebSocket Functions =====

  function openWebSocket(url) {
    // Close existing connection if any
    if (wsConnection) {
      wsConnection.close();
      wsConnection = null;
    }

    // Build WebSocket URL
    let wsUrl;
    if (url.startsWith('ws://') || url.startsWith('wss://')) {
      wsUrl = url;
    } else if (url.startsWith('/')) {
      // Relative path - convert to ws/wss based on current page protocol
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      wsUrl = `${proto}//${window.location.host}${url}`;
    } else if (url.startsWith('http://')) {
      wsUrl = url.replace('http:', 'ws:');
    } else if (url.startsWith('https://')) {
      wsUrl = url.replace('https:', 'wss:');
    } else {
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      wsUrl = `${proto}//${url}`;
    }

    // Build query params from the params container
    const params = getKeyValuePairs(queryParamsContainer);
    const searchParts = [];
    Object.keys(params).forEach(k => {
      if (params[k]) searchParts.push(`${encodeURIComponent(k)}=${encodeURIComponent(params[k])}`);
    });

    // Add auth: real JWT token or dev bypass token (always included for WS)
    const wsToken = authPresetCheck.checked ? authTokenInput.value.trim() : '';
    if (wsToken) {
      searchParts.push(`token=${encodeURIComponent(wsToken)}`);
    } else {
      // Default dev bypass: allows testing without a real JWT token
      searchParts.push('dev_token=nowled2_token');
    }

    if (searchParts.length > 0) {
      wsUrl += (wsUrl.includes('?') ? '&' : '?') + searchParts.join('&');
    }

    // Switch UI to WebSocket console view
    responsePlaceholder.style.display = 'none';
    responseContent.style.display = 'none';
    wsConsole.style.display = 'flex';

    // Clear previous messages
    wsMessages.innerHTML = '';
    appendWsMsg(`Connecting to ${wsUrl}...`, 'system');

    // Update status
    setWsStatus('connecting');
    btnSend.disabled = true;
    btnSend.innerHTML = `<span class="spinner"></span> Connecting...`;
    btnWsClose.style.display = 'block';
    btnWsSend.disabled = true;
    wsMessageInput.disabled = true;

    try {
      wsConnection = new WebSocket(wsUrl);
    } catch (e) {
      appendWsMsg(`Failed to create connection: ${e.message}`, 'error');
      setWsStatus('disconnected');
      resetSendButton();
      return;
    }

    wsConnection.onopen = () => {
      appendWsMsg('Connected successfully.', 'system');
      setWsStatus('connected');
      btnSend.disabled = false;
      btnSend.innerHTML = `
        <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <line x1="22" y1="2" x2="11" y2="13"></line>
          <polygon points="22 2 15 22 11 13 2 9 22 2"></polygon>
        </svg> Send
      `;
      btnWsSend.disabled = false;
      wsMessageInput.disabled = false;
      wsMessageInput.focus();
    };

    wsConnection.onmessage = (event) => {
      let raw = event.data;
      let parsed;
      try {
        parsed = JSON.parse(raw);
      } catch (e) {
        appendWsMsg(raw, 'received');
        return;
      }

      if (parsed.type === 'done' && parsed.result) {
        appendWsMsg(`--- Full Result ---\n${parsed.result}`, 'result');
      } else {
        data = JSON.stringify(parsed, null, 2);
        appendWsMsg(data, 'received');
      }
    };

    wsConnection.onclose = (event) => {
      const reason = event.reason ? ` (${event.reason})` : '';
      const code = event.code ? ` Code: ${event.code}` : '';
      appendWsMsg(`Connection closed.${code}${reason}`, 'system');
      setWsStatus('disconnected');
      btnWsSend.disabled = true;
      wsMessageInput.disabled = true;
      btnWsClose.style.display = 'none';
      wsConnection = null;
    };

    wsConnection.onerror = (event) => {
      appendWsMsg('Connection error occurred.', 'error');
      setWsStatus('disconnected');
    };
  }

  function closeWebSocket() {
    if (wsConnection) {
      appendWsMsg('Closing connection...', 'system');
      wsConnection.close(1000, 'Closed by user');
      wsConnection = null;
    }
    btnWsSend.disabled = true;
    wsMessageInput.disabled = true;
  }

  function sendWsMessage() {
    if (!wsConnection || wsConnection.readyState !== WebSocket.OPEN) {
      appendWsMsg('Cannot send: connection is not open.', 'error');
      return;
    }

    let msg = wsMessageInput.value.trim();
    if (!msg) return;

    // If input is not valid JSON, auto-wrap into expected chat payload format
    try {
      JSON.parse(msg);
    } catch (e) {
      msg = JSON.stringify({ type: 'message', dreamContent: msg });
    }

    wsConnection.send(msg);
    appendWsMsg(msg, 'sent');
    wsMessageInput.value = '';
  }

  function appendWsMsg(text, type) {
    const msgEl = document.createElement('div');
    msgEl.className = `ws-msg ${type}`;

    const timeStr = new Date().toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });

    msgEl.innerHTML = `<span class="ws-msg-time">[${timeStr}]</span>${escapeHtml(text)}`;
    wsMessages.appendChild(msgEl);

    // Auto-scroll to bottom
    wsMessages.scrollTop = wsMessages.scrollHeight;
  }

  function setWsStatus(status) {
    wsStatusDot.className = 'ws-status-dot';

    switch (status) {
      case 'connecting':
        wsStatusDot.classList.add('connecting');
        wsStatusText.textContent = 'Connecting...';
        break;
      case 'connected':
        wsStatusDot.classList.add('connected');
        wsStatusText.textContent = 'Connected';
        break;
      default:
        wsStatusText.textContent = 'Disconnected';
    }
  }

  function resetSendButton() {
    btnSend.disabled = false;
    btnSend.innerHTML = `
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <line x1="22" y1="2" x2="11" y2="13"></line>
        <polygon points="22 2 15 22 11 13 2 9 22 2"></polygon>
      </svg> Send
    `;
  }

  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }
});
