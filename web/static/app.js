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
})();
