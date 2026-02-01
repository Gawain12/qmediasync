(() => {
  const API = '/api/scrape/tmdb/search';
  const PANEL_ID = 'tmdb-candidates-panel';
  const STYLE_ID = 'tmdb-candidates-style';

  const ensureStyle = () => {
    if (document.getElementById(STYLE_ID)) return;
    const style = document.createElement('style');
    style.id = STYLE_ID;
    style.textContent = `
      #${PANEL_ID} { margin-top: 12px; padding: 10px; border: 1px dashed #3b4a6b; border-radius: 8px; background: #0b1226; }
      #${PANEL_ID} .tmdb-actions { display:flex; gap:8px; flex-wrap:wrap; margin-bottom:8px; }
      #${PANEL_ID} .tmdb-list { display:grid; grid-template-columns: repeat(auto-fill,minmax(120px,1fr)); gap:10px; }
      #${PANEL_ID} .tmdb-card { border: 1px solid #1f2a44; border-radius: 10px; overflow: hidden; background: #0f172a; display:flex; flex-direction:column; }
      #${PANEL_ID} .tmdb-poster { height: 64px; background:#0b1226; display:flex; align-items:center; justify-content:center; color:#94a3b8; font-size:9px; }
      #${PANEL_ID} .tmdb-poster img { width:100%; height:100%; object-fit:cover; }
      #${PANEL_ID} .tmdb-body { padding:8px; display:flex; flex-direction:column; gap:4px; }
      #${PANEL_ID} .tmdb-title { font-weight:700; font-size:13px; color:#e2e8f0; }
      #${PANEL_ID} .tmdb-meta { font-size:11px; color:#94a3b8; }
      #${PANEL_ID} .tmdb-btn { padding:4px 8px; font-size:12px; background:#38bdf8; border:none; border-radius:6px; cursor:pointer; color:#02131d; }
      #${PANEL_ID} .tmdb-link { font-size:11px; color:#38bdf8; text-decoration:none; }
    `;
    document.head.appendChild(style);
  };

  const findDialog = () => {
    const dialogs = Array.from(document.querySelectorAll('.el-dialog'));
    return dialogs.find(d => d.textContent && (d.textContent.includes('重新识别') || d.textContent.includes('重新刮削')));
  };

  const findFormItemByLabel = (dialog, keywords) => {
    const labels = dialog.querySelectorAll('label, .el-form-item__label');
    for (const label of labels) {
      const text = (label.textContent || '').trim();
      if (!text) continue;
      if (keywords.some(k => text.includes(k))) {
        const item = label.closest('.el-form-item');
        if (item) return item;
      }
    }
    return null;
  };

  const findInput = (dialog, keywords) => {
    const item = findFormItemByLabel(dialog, keywords);
    if (item) {
      const input = item.querySelector('input');
      if (input) return input;
    }
    // fallback: placeholder contains keyword
    const inputs = dialog.querySelectorAll('input');
    for (const input of inputs) {
      const ph = (input.getAttribute('placeholder') || '').trim();
      if (ph && keywords.some(k => ph.includes(k))) return input;
    }
    return null;
  };

  const findFilename = (dialog) => {
    const item = findFormItemByLabel(dialog, ['文件名', 'Filename']);
    if (!item) return '';
    const text = (item.textContent || '').trim();
    // strip label and keep filename
    return text.replace(/^文件名[:：]?\s*/i, '').trim();
  };

  const parseNameYearFromFilename = (filename) => {
    if (!filename) return { name: '', year: '' };
    const raw = filename.replace(/^\s+|\s+$/g, '');
    const noExt = raw.replace(/\.[a-z0-9]{2,4}$/i, '');
    // keep original dots for better split, but normalize spaces
    const normalized = noExt.replace(/[_]+/g, ' ').replace(/\s+/g, ' ').trim();
    const yearMatch = normalized.match(/\b(19\d{2}|20\d{2})\b/);
    let year = yearMatch ? yearMatch[1] : '';
    let name = normalized;
    if (yearMatch) {
      name = normalized.slice(0, yearMatch.index).trim();
    }
    // fallback: split by dots if still empty
    if (!name) {
      const dotSplit = noExt.split('.');
      const yIdx = dotSplit.findIndex(part => /^(19\d{2}|20\d{2})$/.test(part));
      if (yIdx > 0) {
        name = dotSplit.slice(0, yIdx).join(' ').trim();
        year = dotSplit[yIdx];
      } else {
        name = dotSplit[0] ? dotSplit[0].trim() : '';
      }
    }
    name = name.replace(/[._]+/g, ' ').replace(/\s+/g, ' ').trim();
    return { name, year };
  };

  const setInputValue = (input, value) => {
    if (!input) return;
    input.value = value;
    input.dispatchEvent(new Event('input', { bubbles: true }));
    input.dispatchEvent(new Event('change', { bubbles: true }));
  };

  const getAuthToken = () => {
    try {
      return localStorage.getItem('auth_token') || sessionStorage.getItem('auth_token') || '';
    } catch (_) {
      return '';
    }
  };

  const getMediaType = (dialog) => {
    const seasonInput = findInput(dialog, ['季', 'Season']);
    const episodeInput = findInput(dialog, ['集', 'Episode']);
    const season = seasonInput ? parseInt(seasonInput.value || '0', 10) : 0;
    const episode = episodeInput ? parseInt(episodeInput.value || '0', 10) : 0;
    return season > 0 || episode > 0 ? 'tvshow' : 'movie';
  };

  const findRecordId = (dialog) => {
    const idInput = findInput(dialog, ['ID', '记录ID', '条目ID']);
    if (idInput) {
      const val = (idInput.value || '').trim();
      if (/^\d+$/.test(val)) return val;
    }
    const text = dialog.textContent || '';
    const match = text.match(/ID[:：]\s*(\d+)/i);
    return match ? match[1] : '';
  };

  const renderCandidates = (panel, items, onPick) => {
    const list = panel.querySelector('.tmdb-list');
    list.innerHTML = '';
    if (!items || items.length === 0) {
      list.innerHTML = '<div style="color:#94a3b8;font-size:12px;">没有结果</div>';
      return;
    }
    items.forEach(item => {
      const card = document.createElement('div');
      card.className = 'tmdb-card';
      const poster = document.createElement('div');
      poster.className = 'tmdb-poster';
      if (item.poster) {
        const img = document.createElement('img');
        img.src = item.poster;
        img.alt = item.name || '';
        poster.appendChild(img);
      } else {
        poster.textContent = 'No poster';
      }
      const body = document.createElement('div');
      body.className = 'tmdb-body';
      const title = document.createElement('div');
      title.className = 'tmdb-title';
      title.textContent = item.name || '(no title)';
      const meta = document.createElement('div');
      meta.className = 'tmdb-meta';
      meta.textContent = `TMDB ${item.tmdb_id || '-'} | ${item.year || '-'}`;
      const link = document.createElement('a');
      link.className = 'tmdb-link';
      link.href = item.tmdb_url || '#';
      link.target = '_blank';
      link.rel = 'noreferrer';
      link.textContent = 'Open TMDB';

      const btn = document.createElement('button');
      btn.className = 'tmdb-btn';
      btn.textContent = 'Use';
      btn.addEventListener('click', () => onPick(item));

      body.appendChild(title);
      body.appendChild(meta);
      if (item.tmdb_url) body.appendChild(link);
      body.appendChild(btn);
      card.appendChild(poster);
      card.appendChild(body);
      list.appendChild(card);
    });
  };

  const ensurePanel = (dialog) => {
    let panel = dialog.querySelector(`#${PANEL_ID}`);
    if (panel) return panel;
    ensureStyle();
    panel = document.createElement('div');
    panel.id = PANEL_ID;
    panel.innerHTML = `
      <div class="tmdb-actions">
        <button class="tmdb-btn" id="tmdb-search-btn">搜索 TMDB</button>
        <button class="tmdb-btn" id="tmdb-clear-btn" style="background:#f59e0b;">清空结果</button>
        <span id="tmdb-status" style="font-size:12px;color:#94a3b8;">待搜索</span>
      </div>
      <div class="tmdb-list"></div>
    `;
    const form = dialog.querySelector('.el-form') || dialog.querySelector('form') || dialog.querySelector('.el-dialog__body');
    if (form) {
      form.appendChild(panel);
    } else {
      dialog.appendChild(panel);
    }
    return panel;
  };

  const fetchRecord = async (id) => {
    if (!id) return null;
    const headers = { 'Content-Type': 'application/json' };
    const token = getAuthToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(`/api/scrape/record?id=${encodeURIComponent(id)}`, { headers });
    const json = await res.json();
    if (json.code !== 200) return null;
    return json.data || null;
  };

  const fetchCandidates = async (name, year, mediaType, tmdbId) => {
    const params = new URLSearchParams();
    if (name) params.set('name', name);
    if (year) params.set('year', year);
    if (mediaType) params.set('media_type', mediaType);
    if (tmdbId) params.set('tmdb_id', tmdbId);
    const url = `${API}?${params.toString()}`;
    const headers = { 'Content-Type': 'application/json' };
    const token = getAuthToken();
    if (token) headers.Authorization = `Bearer ${token}`;
    const res = await fetch(url, { headers });
    const json = await res.json();
    if (json.code !== 200) throw new Error(json.message || '查询失败');
    return json.data.items || [];
  };

  const attach = (dialog) => {
    const panel = ensurePanel(dialog);
    const status = panel.querySelector('#tmdb-status');
    const btn = panel.querySelector('#tmdb-search-btn');
    const clearBtn = panel.querySelector('#tmdb-clear-btn');

    const nameInput = findInput(dialog, ['名称', 'Name', '片名']);
    const yearInput = findInput(dialog, ['年份', 'Year']);
    const tmdbInput = findInput(dialog, ['TMDB', 'tmdb']);
    const recordId = findRecordId(dialog);
    const filename = findFilename(dialog);

    const pickItem = (item) => {
      if (nameInput && item.name) setInputValue(nameInput, item.name);
      if (yearInput && item.year) setInputValue(yearInput, item.year);
      if (tmdbInput && item.tmdb_id) setInputValue(tmdbInput, item.tmdb_id);
      status.textContent = '已填入 TMDB 信息';
    };

    const prefillFromRecord = async () => {
      if (!recordId) return;
      try {
        const record = await fetchRecord(recordId);
        if (!record) return;
        if (nameInput && !nameInput.value && record.name) setInputValue(nameInput, record.name);
        if (yearInput && (!yearInput.value || yearInput.value === '0') && record.year) setInputValue(yearInput, record.year);
        if (tmdbInput && !tmdbInput.value && record.tmdb_id) setInputValue(tmdbInput, record.tmdb_id);
      } catch (_) {
        // ignore
      }
    };

    const prefillFromFilename = () => {
      if (!nameInput) return;
      const parsed = parseNameYearFromFilename(findFilename(dialog));
      if (parsed.name && !nameInput.value) setInputValue(nameInput, parsed.name);
      if (yearInput) {
        const existingYear = (yearInput.value || '').trim();
        if ((existingYear === '' || existingYear === '0') && parsed.year) {
          setInputValue(yearInput, parsed.year);
        }
      }
    };

    let lastFilename = '';
    const refresh = async () => {
      // clear prior candidates on each open
      renderCandidates(panel, [], pickItem);
      status.textContent = '待搜索';
      await prefillFromRecord();
      prefillFromFilename();
    };

    const maybeRefresh = () => {
      const current = findFilename(dialog);
      if (current && current !== lastFilename) {
        lastFilename = current;
        refresh();
      }
    };

    refresh();
    maybeRefresh();

    btn.onclick = async () => {
      try {
        const name = nameInput ? (nameInput.value || '').trim() : '';
        const tmdbId = tmdbInput ? (tmdbInput.value || '').trim() : '';
        if (!name && !tmdbId) {
        status.textContent = '请先填写名称或 TMDB ID';
        return;
      }
      status.textContent = '搜索中...';
      const year = yearInput ? (yearInput.value || '').trim() : '';
      const mediaType = getMediaType(dialog);
      const items = await fetchCandidates(name, year, mediaType, tmdbId);
      renderCandidates(panel, items, pickItem);
      status.textContent = `找到 ${items.length} 个结果`;
    } catch (err) {
        status.textContent = err.message || '搜索失败';
      }
    };

    clearBtn.onclick = () => {
      renderCandidates(panel, [], pickItem);
      status.textContent = '已清空';
    };

    // re-run prefill when dialog is re-opened (element reused)
    const wrapper = dialog.closest('.el-dialog__wrapper') || dialog.parentElement;
    if (wrapper && !wrapper.__tmdbObserve) {
      wrapper.__tmdbObserve = true;
      const observer = new MutationObserver(() => {
        const style = window.getComputedStyle(wrapper);
        if (style.display !== 'none' && style.visibility !== 'hidden') {
          maybeRefresh();
        }
      });
      observer.observe(wrapper, { attributes: true, attributeFilter: ['style', 'class'] });
    }

    if (!dialog.__tmdbFilenameObserve) {
      dialog.__tmdbFilenameObserve = true;
      const filenameObserver = new MutationObserver(() => {
        maybeRefresh();
      });
      filenameObserver.observe(dialog, { childList: true, subtree: true, characterData: true });
    }

    [btn, clearBtn].forEach(button => {
      if (!button) return;
      button.type = 'button';
      button.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
      });
    });
  };

  const observe = () => {
    const observer = new MutationObserver(() => {
      const dialog = findDialog();
      if (dialog && !dialog.__tmdbAttached) {
        dialog.__tmdbAttached = true;
        attach(dialog);
      }
    });
    observer.observe(document.body, { childList: true, subtree: true });
  };

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', observe);
  } else {
    observe();
  }
})();
