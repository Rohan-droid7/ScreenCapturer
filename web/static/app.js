(function () {
  'use strict';

  // Elements
  const captureBtn = document.getElementById('captureBtn');
  const captureSpinner = document.getElementById('captureSpinner');
  const captureText = document.getElementById('captureText');
  const formatSelect = document.getElementById('formatSelect');
  const previewSection = document.getElementById('previewSection');
  const previewImage = document.getElementById('previewImage');
  const downloadLink = document.getElementById('downloadLink');
  const shareBtn = document.getElementById('shareBtn');
  const captureMeta = document.getElementById('captureMeta');
  const timingInfo = document.getElementById('timingInfo');
  const errorBanner = document.getElementById('errorBanner');
  const errorMessage = document.getElementById('errorMessage');
  const dismissError = document.getElementById('dismissError');
  const tokenBanner = document.getElementById('tokenBanner');
  const manualTokenInput = document.getElementById('manualTokenInput');
  const saveTokenBtn = document.getElementById('saveTokenBtn');
  const turnOffBtn = document.getElementById('turnOffBtn');
  const lockBtn = document.getElementById('lockBtn');
  const lockSpinner = document.getElementById('lockSpinner');
  const lockText = document.getElementById('lockText');
  const noticeBanner = document.getElementById('noticeBanner');
  const noticeMessage = document.getElementById('noticeMessage');
  const dismissNotice = document.getElementById('dismissNotice');
  const clipInput = document.getElementById('clipInput');
  const clipMeta = document.getElementById('clipMeta');
  const getClipBtn = document.getElementById('getClipBtn');
  const getClipSpinner = document.getElementById('getClipSpinner');
  const getClipText = document.getElementById('getClipText');
  const sendClipBtn = document.getElementById('sendClipBtn');
  const sendClipSpinner = document.getElementById('sendClipSpinner');
  const sendClipText = document.getElementById('sendClipText');
  const copyPhoneBtn = document.getElementById('copyPhoneBtn');
  const clearClipBtn = document.getElementById('clearClipBtn');
  const historySection = document.getElementById('historySection');
  const historyCount = document.getElementById('historyCount');
  const filmstripTrack = document.getElementById('filmstripTrack');
  const clearHistoryBtn = document.getElementById('clearHistoryBtn');

  let currentBlobUrl = null;
  let currentFile = null;

  // 1. Fragment Token Handling (#token=<token>)
  function checkUrlFragmentToken() {
    const hash = window.location.hash;
    if (hash && hash.includes('token=')) {
      const match = hash.match(/token=([a-fA-F0-9]+)/);
      if (match && match[1]) {
        localStorage.setItem('screencap_token', match[1]);
        // Strip token fragment from address bar cleanly
        history.replaceState(null, '', window.location.pathname + window.location.search);
      }
    }
  }

  function getToken() {
    return localStorage.getItem('screencap_token') || '';
  }

  function updateTokenBanner() {
    const token = getToken();
    if (!token) {
      tokenBanner.classList.remove('hidden');
    } else {
      tokenBanner.classList.add('hidden');
    }
  }

  saveTokenBtn.addEventListener('click', function () {
    const val = manualTokenInput.value.trim();
    if (val) {
      localStorage.setItem('screencap_token', val);
      manualTokenInput.value = '';
      updateTokenBanner();
      hideError();
    }
  });

  dismissError.addEventListener('click', hideError);

  function showNotice(msg) {
    if (noticeMessage && noticeBanner) {
      noticeMessage.textContent = msg;
      noticeBanner.classList.remove('hidden');
      setTimeout(() => hideNotice(), 6000);
    }
  }

  function hideNotice() {
    if (noticeBanner) noticeBanner.classList.add('hidden');
  }

  if (dismissNotice) {
    dismissNotice.addEventListener('click', hideNotice);
  }

  function showError(msg) {
    errorMessage.textContent = msg;
    errorBanner.classList.remove('hidden');
  }

  function hideError() {
    errorBanner.classList.add('hidden');
  }

  // Check if Web Share API with files is supported
  function canShareFiles() {
    return navigator.canShare && typeof File !== 'undefined';
  }

  // 2. Perform Capture Request
  async function performCapture() {
    hideError();
    const token = getToken();
    if (!token) {
      showError('No access token found. Please pair using the QR code or enter your token.');
      updateTokenBanner();
      return;
    }

    const format = formatSelect.value || 'png';
    const startTime = performance.now();

    // UI loading state
    captureBtn.disabled = true;
    captureSpinner.classList.remove('hidden');
    captureText.textContent = 'Capturing screen...';

    try {
      const resp = await fetch(`/api/capture?format=${encodeURIComponent(format)}`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });

      const roundTripMs = Math.round(performance.now() - startTime);

      if (!resp.ok) {
        let errDesc = `Capture failed (${resp.status})`;
        try {
          const json = await resp.json();
          if (json && json.error) {
            errDesc = json.error;
          }
        } catch (_) {
          const text = await resp.text();
          if (text) errDesc = text;
        }

        if (resp.status === 401 || resp.status === 403) {
          showError(`Authentication Error: ${errDesc}. Check or regenerate your pairing token.`);
          updateTokenBanner();
        } else if (resp.status === 429) {
          showError(`Rate Limited: ${errDesc}`);
        } else if (resp.status === 503 || (errDesc && errDesc.toLowerCase().includes('locked'))) {
          showError(`🔒 Computer is locked (${errDesc}). Unlock on PC to resume captures.`);
        } else {
          showError(`Error: ${errDesc}`);
        }
        return;
      }

      const blob = await resp.blob();
      const ext = format === 'jpeg' ? 'jpg' : 'png';
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
      const filename = `screenshot-${timestamp}.${ext}`;

      // Revoke previous blob URL to avoid memory leak
      if (currentBlobUrl) {
        URL.revokeObjectURL(currentBlobUrl);
      }
      currentBlobUrl = URL.createObjectURL(blob);
      previewImage.src = currentBlobUrl;

      // Update download link
      downloadLink.href = currentBlobUrl;
      downloadLink.download = filename;

      // Format file size
      const sizeKB = Math.round(blob.size / 1024);
      captureMeta.textContent = `${ext.toUpperCase()} • ${sizeKB} KB • ${roundTripMs} ms roundtrip`;
      timingInfo.textContent = `Delivered in ${roundTripMs} ms`;

      // Save to Capture History Filmstrip
      const timeFormatted = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
      addCaptureToHistory({
        id: Date.now(),
        timestamp: new Date().toISOString(),
        timeFormatted: timeFormatted,
        format: format,
        sizeKB: sizeKB,
        roundTripMs: roundTripMs,
        blob: blob,
        filename: filename
      });

      // Web share setup
      if (canShareFiles()) {
        try {
          currentFile = new File([blob], filename, { type: blob.type });
          if (navigator.canShare({ files: [currentFile] })) {
            shareBtn.classList.remove('hidden');
          } else {
            shareBtn.classList.add('hidden');
          }
        } catch (_) {
          shareBtn.classList.add('hidden');
        }
      }

      previewSection.classList.remove('hidden');
      previewSection.scrollIntoView({ behavior: 'smooth', block: 'nearest' });

    } catch (err) {
      showError(`Network connection error: ${err.message || 'Server unreachable'}`);
    } finally {
      captureBtn.disabled = false;
      captureSpinner.classList.add('hidden');
      captureText.textContent = '📸 Capture Screen';
    }
  }

  captureBtn.addEventListener('click', performCapture);

  shareBtn.addEventListener('click', async function () {
    if (!currentFile || !navigator.share) return;
    try {
      await navigator.share({
        files: [currentFile],
        title: 'Desktop Screenshot'
      });
    } catch (err) {
      if (err.name !== 'AbortError') {
        showError(`Sharing failed: ${err.message}`);
      }
    }
  });

    if (lockBtn) {
    lockBtn.addEventListener('click', async function () {
      const token = getToken();
      if (!token) {
        showError('No access token found. Please pair using the QR code or enter your token.');
        updateTokenBanner();
        return;
      }

      if (!confirm('Lock your computer workstation now (Win + L / Panic Lock)?\n\nYour session will be locked immediately.')) {
        return;
      }

      hideError();
      hideNotice();

      lockBtn.disabled = true;
      if (lockSpinner) lockSpinner.classList.remove('hidden');
      if (lockText) lockText.textContent = 'Locking...';

      try {
        const resp = await fetch('/api/lock', {
          method: 'POST',
          headers: {
            'Authorization': 'Bearer ' + token
          }
        });

        if (!resp.ok) {
          let errDesc = 'Lock failed (' + resp.status + ')';
          try {
            const json = await resp.json();
            if (json && json.error) errDesc = json.error;
          } catch (_) {}
          showError('Failed to lock: ' + errDesc);
          return;
        }

        showNotice('🔒 Workstation locked successfully! (Win + L applied)');
        if (lockText) lockText.textContent = '✅ Locked!';
        setTimeout(() => {
          if (lockText) lockText.textContent = '🔒 Lock PC';
        }, 3000);
      } catch (err) {
        showError('Network connection error: ' + (err.message || 'Server unreachable'));
      } finally {
        lockBtn.disabled = false;
        if (lockSpinner) lockSpinner.classList.add('hidden');
        if (lockText && lockText.textContent !== '✅ Locked!') {
          lockText.textContent = '🔒 Lock PC';
        }
      }
    });
  }

  // 3. Clipboard Sync Handlers
  if (getClipBtn) {
    getClipBtn.addEventListener('click', async function () {
      const token = getToken();
      if (!token) {
        showError('No access token found. Please pair using the QR code or enter your token.');
        updateTokenBanner();
        return;
      }

      hideError();
      getClipBtn.disabled = true;
      if (getClipSpinner) getClipSpinner.classList.remove('hidden');
      if (getClipText) getClipText.textContent = 'Fetching...';

      try {
        const resp = await fetch('/api/clipboard', {
          headers: { 'Authorization': 'Bearer ' + token }
        });

        if (!resp.ok) {
          let errDesc = 'Failed to fetch clipboard (' + resp.status + ')';
          try {
            const json = await resp.json();
            if (json && json.error) errDesc = json.error;
          } catch (_) {}
          showError(errDesc);
          return;
        }

        const data = await resp.json();
        if (data.has_text) {
          if (clipInput) clipInput.value = data.text;
          if (clipMeta) clipMeta.textContent = data.length + ' chars';
          showNotice('📋 PC clipboard grabbed (' + data.length + ' chars)!');
          if (copyPhoneBtn) copyPhoneBtn.classList.remove('hidden');
        } else {
          if (clipInput) clipInput.value = '';
          if (clipMeta) clipMeta.textContent = 'Empty';
          showNotice('ℹ️ PC clipboard is currently empty.');
        }
      } catch (err) {
        showError('Network error: ' + (err.message || 'Server unreachable'));
      } finally {
        getClipBtn.disabled = false;
        if (getClipSpinner) getClipSpinner.classList.add('hidden');
        if (getClipText) getClipText.textContent = '📥 Get PC Clipboard';
      }
    });
  }

  if (sendClipBtn) {
    sendClipBtn.addEventListener('click', async function () {
      const token = getToken();
      if (!token) {
        showError('No access token found. Please pair using the QR code or enter your token.');
        updateTokenBanner();
        return;
      }

      const text = clipInput ? clipInput.value : '';
      if (!text || text.trim() === '') {
        showError('Please type or paste text into the clipboard box before sending to PC.');
        if (clipInput) clipInput.focus();
        return;
      }

      hideError();
      sendClipBtn.disabled = true;
      if (sendClipSpinner) sendClipSpinner.classList.remove('hidden');
      if (sendClipText) sendClipText.textContent = 'Sending...';

      try {
        const resp = await fetch('/api/clipboard', {
          method: 'POST',
          headers: {
            'Authorization': 'Bearer ' + token,
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({ text: text })
        });

        if (!resp.ok) {
          let errDesc = 'Failed to set clipboard (' + resp.status + ')';
          try {
            const json = await resp.json();
            if (json && json.error) errDesc = json.error;
          } catch (_) {}
          showError(errDesc);
          return;
        }

        showNotice('✅ Sent to PC clipboard! Ready to Ctrl + V on your laptop.');
        if (sendClipText) sendClipText.textContent = '✅ Sent!';
        setTimeout(() => {
          if (sendClipText) sendClipText.textContent = '📤 Send to PC';
        }, 2500);
      } catch (err) {
        showError('Network error: ' + (err.message || 'Server unreachable'));
      } finally {
        sendClipBtn.disabled = false;
        if (sendClipSpinner) sendClipSpinner.classList.add('hidden');
        if (sendClipText && sendClipText.textContent !== '✅ Sent!') {
          sendClipText.textContent = '📤 Send to PC';
        }
      }
    });
  }

  if (copyPhoneBtn) {
    copyPhoneBtn.addEventListener('click', async function () {
      if (!clipInput || !clipInput.value) return;
      try {
        if (navigator.clipboard && navigator.clipboard.writeText) {
          await navigator.clipboard.writeText(clipInput.value);
          showNotice('📋 Copied to phone clipboard!');
        } else {
          clipInput.select();
          document.execCommand('copy');
          showNotice('📋 Copied to phone clipboard!');
        }
      } catch (err) {
        showError('Could not copy to phone: ' + err.message);
      }
    });
  }

  if (clearClipBtn) {
    clearClipBtn.addEventListener('click', function () {
      if (clipInput) clipInput.value = '';
      if (clipMeta) clipMeta.textContent = '2-Way Sync';
      if (copyPhoneBtn) copyPhoneBtn.classList.add('hidden');
    });
  }


  // ==========================================
  // Capture History Filmstrip (IndexedDB + Memory)
  // ==========================================
  const MAX_HISTORY = 10;
  let captureHistory = [];
  let activeHistoryId = null;
  const historyUrls = new Map();

  const DB_NAME = 'ScreenCapturerDB';
  const DB_VERSION = 1;
  const STORE_NAME = 'captures';
  let dbPromise = null;

  function getDB() {
    if (dbPromise) return dbPromise;
    dbPromise = new Promise((resolve) => {
      if (!('indexedDB' in window)) {
        resolve(null);
        return;
      }
      try {
        const req = indexedDB.open(DB_NAME, DB_VERSION);
        req.onupgradeneeded = function (e) {
          const db = e.target.result;
          if (!db.objectStoreNames.contains(STORE_NAME)) {
            db.createObjectStore(STORE_NAME, { keyPath: 'id' });
          }
        };
        req.onsuccess = function (e) {
          resolve(e.target.result);
        };
        req.onerror = function () {
          resolve(null);
        };
      } catch (_) {
        resolve(null);
      }
    });
    return dbPromise;
  }

  async function loadHistoryFromStorage() {
    try {
      const db = await getDB();
      if (!db) return;
      const tx = db.transaction(STORE_NAME, 'readonly');
      const store = tx.objectStore(STORE_NAME);
      const req = store.getAll();
      req.onsuccess = function () {
        const items = req.result || [];
        items.sort((a, b) => b.id - a.id);
        captureHistory = items.slice(0, MAX_HISTORY);
        if (captureHistory.length > 0) {
          renderFilmstrip(captureHistory[0].id);
          selectHistoryItem(captureHistory[0].id, false);
        }
      };
    } catch (_) {}
  }

  async function persistCapture(item) {
    try {
      const db = await getDB();
      if (!db) return;
      const tx = db.transaction(STORE_NAME, 'readwrite');
      const store = tx.objectStore(STORE_NAME);
      store.put(item);

      const getAllReq = store.getAllKeys();
      getAllReq.onsuccess = function () {
        const keys = getAllReq.result || [];
        if (keys.length > MAX_HISTORY) {
          keys.sort((a, b) => b - a);
          for (let i = MAX_HISTORY; i < keys.length; i++) {
            store.delete(keys[i]);
          }
        }
      };
    } catch (_) {}
  }

  async function removeCaptureFromStorage(id) {
    try {
      const db = await getDB();
      if (!db) return;
      const tx = db.transaction(STORE_NAME, 'readwrite');
      tx.objectStore(STORE_NAME).delete(id);
    } catch (_) {}
  }

  async function clearStorage() {
    try {
      const db = await getDB();
      if (!db) return;
      const tx = db.transaction(STORE_NAME, 'readwrite');
      tx.objectStore(STORE_NAME).clear();
    } catch (_) {}
  }

  function getBlobUrl(item) {
    if (!historyUrls.has(item.id)) {
      historyUrls.set(item.id, URL.createObjectURL(item.blob));
    }
    return historyUrls.get(item.id);
  }

  function renderFilmstrip(selectedId) {
    if (!historySection || !filmstripTrack) return;

    if (captureHistory.length === 0) {
      historySection.classList.add('hidden');
      return;
    }

    historySection.classList.remove('hidden');
    if (historyCount) {
      historyCount.textContent = captureHistory.length + '/' + MAX_HISTORY;
    }

    const currentIds = new Set(captureHistory.map(i => i.id));
    for (const [id, url] of historyUrls.entries()) {
      if (!currentIds.has(id)) {
        URL.revokeObjectURL(url);
        historyUrls.delete(id);
      }
    }

    filmstripTrack.innerHTML = '';
    captureHistory.forEach(item => {
      const url = getBlobUrl(item);
      const div = document.createElement('div');
      div.className = 'filmstrip-item' + (item.id === selectedId ? ' active' : '');
      div.setAttribute('data-id', item.id);
      div.setAttribute('role', 'listitem');

      div.innerHTML = `
        <div class="thumb-frame">
          <img src="${url}" alt="Capture ${item.timeFormatted}" loading="lazy" />
          <span class="thumb-badge">${item.timeFormatted}</span>
        </div>
        <button class="thumb-del-btn" title="Remove capture" data-del-id="${item.id}">&times;</button>
      `;

      div.addEventListener('click', function (e) {
        if (e.target.closest('.thumb-del-btn')) {
          e.stopPropagation();
          deleteHistoryItem(item.id);
          return;
        }
        selectHistoryItem(item.id, true);
      });

      filmstripTrack.appendChild(div);
    });
  }

  function selectHistoryItem(id, scrollIntoView) {
    activeHistoryId = id;
    const item = captureHistory.find(i => i.id === id);
    if (!item) return;

    document.querySelectorAll('.filmstrip-item').forEach(el => {
      el.classList.toggle('active', el.getAttribute('data-id') == id);
    });

    const url = getBlobUrl(item);
    previewImage.src = url;
    downloadLink.href = url;
    downloadLink.download = item.filename;
    captureMeta.textContent = item.format.toUpperCase() + ' • ' + item.sizeKB + ' KB • ' + item.timeFormatted;
    timingInfo.textContent = 'Viewing capture from ' + item.timeFormatted;

    if (canShareFiles()) {
      try {
        currentFile = new File([item.blob], item.filename, { type: item.blob.type });
        if (navigator.canShare && navigator.canShare({ files: [currentFile] })) {
          shareBtn.classList.remove('hidden');
        } else {
          shareBtn.classList.add('hidden');
        }
      } catch (_) {
        shareBtn.classList.add('hidden');
      }
    }

    previewSection.classList.remove('hidden');
    if (scrollIntoView) {
      previewSection.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }
  }

  function addCaptureToHistory(item) {
    captureHistory.unshift(item);
    if (captureHistory.length > MAX_HISTORY) {
      const removed = captureHistory.pop();
      if (removed && historyUrls.has(removed.id)) {
        URL.revokeObjectURL(historyUrls.get(removed.id));
        historyUrls.delete(removed.id);
      }
    }
    persistCapture(item);
    renderFilmstrip(item.id);
    activeHistoryId = item.id;
  }

  function deleteHistoryItem(id) {
    const idx = captureHistory.findIndex(i => i.id === id);
    if (idx === -1) return;
    captureHistory.splice(idx, 1);
    if (historyUrls.has(id)) {
      URL.revokeObjectURL(historyUrls.get(id));
      historyUrls.delete(id);
    }
    removeCaptureFromStorage(id);

    if (activeHistoryId === id) {
      if (captureHistory.length > 0) {
        selectHistoryItem(captureHistory[0].id, false);
      } else {
        previewSection.classList.add('hidden');
        previewImage.src = '';
      }
    }
    renderFilmstrip(activeHistoryId);
  }

  if (clearHistoryBtn) {
    clearHistoryBtn.addEventListener('click', function () {
      if (captureHistory.length === 0) return;
      if (!confirm('Clear all recent capture history?')) return;
      for (const url of historyUrls.values()) {
        URL.revokeObjectURL(url);
      }
      historyUrls.clear();
      captureHistory = [];
      clearStorage();
      renderFilmstrip(null);
      previewSection.classList.add('hidden');
      previewImage.src = '';
      showNotice('🗑️ Capture history cleared.');
    });
  }

  if (turnOffBtn) {
    turnOffBtn.addEventListener('click', async function () {
      if (!confirm('Are you sure you want to turn off ScreenCapturer on your laptop?\n\nThe background agent will exit immediately.')) {
        return;
      }
      turnOffBtn.disabled = true;
      turnOffBtn.textContent = 'Stopping...';
      try {
        await fetch('/api/shutdown', {
          method: 'POST',
          headers: { 'Authorization': `Bearer ${getToken()}` }
        });
      } catch (_) {}
      document.body.innerHTML = `
        <div style="font-family: -apple-system, sans-serif; display: flex; flex-direction: column; align-items: center; justify-content: center; min-height: 100vh; background: #0f172a; color: #f8fafc; text-align: center; padding: 20px;">
          <div style="font-size: 3.5rem; margin-bottom: 16px;">🛑</div>
          <h2>ScreenCapturer is Off</h2>
          <p style="color: #94a3b8; margin: 12px 0 20px 0;">The agent has shut down. You can close this page.</p>
        </div>
      `;
    });
  }

  // Initialization
  checkUrlFragmentToken();
  updateTokenBanner();
  loadHistoryFromStorage();
})();
