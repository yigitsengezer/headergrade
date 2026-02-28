document.addEventListener('DOMContentLoaded', () => {
    // Theme logic
    const themeToggleBtn = document.getElementById('theme-toggle');
    const currentTheme = localStorage.getItem('theme');
    if (currentTheme === 'light') {
        document.body.classList.add('light-mode');
    }

    themeToggleBtn.addEventListener('click', () => {
        document.body.classList.toggle('light-mode');
        let theme = 'dark';
        if (document.body.classList.contains('light-mode')) {
            theme = 'light';
        }
        localStorage.setItem('theme', theme);
    });

    const scanForm = document.getElementById('scan-form');
    const urlInput = document.getElementById('url-input');
    const followRedirects = document.getElementById('follow-redirects');
    const scanBtn = document.getElementById('scan-btn');
    const loading = document.getElementById('loading');
    const errorMessage = document.getElementById('error-message');

    const heroSection = document.getElementById('hero-section');
    const resultSection = document.getElementById('result-section');
    const newScanBtn = document.getElementById('new-scan-btn');

    // Result elements
    const gradeDisplay = document.getElementById('grade-display');
    const siteValue = document.getElementById('site-value');
    const ipValue = document.getElementById('ip-value');
    const timeValue = document.getElementById('time-value');
    const missingHeadersContainer = document.getElementById('missing-headers');
    const presentHeadersContainer = document.getElementById('present-headers');
    const rawHeadersContainer = document.getElementById('raw-headers');
    const headersPillsContainer = document.getElementById('headers-pills');

    // Check query params on load
    const urlParams = new URLSearchParams(window.location.search);
    const queryUrl = urlParams.get('url');
    if (queryUrl) {
        urlInput.value = queryUrl;
        const follow = urlParams.get('followRedirects');
        if (follow !== null) {
            followRedirects.checked = follow === 'true';
        }
        performScan(queryUrl, followRedirects.checked);
    }

    scanForm.addEventListener('submit', async (e) => {
        e.preventDefault();

        let url = urlInput.value.trim();
        if (!url) return;

        // Sync main input to quick search
        const quickUrlInput = document.getElementById('quick-url-input');
        if (quickUrlInput) {
            quickUrlInput.value = url;
        }

        // Update URL
        const newUrl = new URL(window.location);
        newUrl.searchParams.set('url', url);
        newUrl.searchParams.set('followRedirects', followRedirects.checked);
        window.history.pushState({}, '', newUrl);

        performScan(url, followRedirects.checked);
    });

    const quickScanForm = document.getElementById('quick-scan-form');
    if (quickScanForm) {
        quickScanForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const quickUrlInput = document.getElementById('quick-url-input');
            let url = quickUrlInput.value.trim();
            if (!url) return;

            // Sync quick search to main input
            urlInput.value = url;

            // Update URL
            const newUrl = new URL(window.location);
            newUrl.searchParams.set('url', url);
            newUrl.searchParams.set('followRedirects', followRedirects.checked);
            window.history.pushState({}, '', newUrl);

            performScan(url, followRedirects.checked);
        });
    }

    async function performScan(url, follow) {
        // Reset state
        errorMessage.classList.add('hidden');
        loading.classList.remove('hidden');
        scanBtn.disabled = true;
        resultSection.classList.add('hidden');
        heroSection.classList.remove('hidden');

        try {
            const response = await fetch('/api/scan', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    url: url,
                    follow_redirects: follow
                })
            });

            const data = await response.json();

            if (data.error) {
                showError(data.error);
                return;
            }

            renderResults(data);

            heroSection.classList.add('hidden');
            resultSection.classList.remove('hidden');

        } catch (err) {
            showError("Network error or server is unreachable. Details: " + err.message);
        } finally {
            loading.classList.add('hidden');
            scanBtn.disabled = false;
        }
    }

    newScanBtn.addEventListener('click', () => {
        resultSection.classList.add('hidden');
        heroSection.classList.remove('hidden');
        urlInput.value = '';

        // Clear URL params
        const newUrl = new URL(window.location);
        newUrl.searchParams.delete('url');
        newUrl.searchParams.delete('followRedirects');
        window.history.pushState({}, '', newUrl);

        urlInput.focus();
    });

    // Handle back/forward browser buttons
    window.addEventListener('popstate', () => {
        const urlParams = new URLSearchParams(window.location.search);
        const queryUrl = urlParams.get('url');
        if (queryUrl) {
            urlInput.value = queryUrl;
            const follow = urlParams.get('followRedirects');
            if (follow !== null) {
                followRedirects.checked = follow === 'true';
            }
            performScan(queryUrl, followRedirects.checked);
        } else {
            resultSection.classList.add('hidden');
            heroSection.classList.remove('hidden');
            urlInput.value = '';
        }
    });

    function showError(msg) {
        errorMessage.textContent = msg;
        errorMessage.classList.remove('hidden');
    }

    function renderResults(data) {
        const siteAnchor = document.getElementById('site-value');
        siteAnchor.textContent = data.site;
        siteAnchor.href = data.site;

        ipValue.textContent = data.ip_address || 'Unknown';
        timeValue.textContent = data.report_time;

        // Format Grade
        let gradeClass = 'grade-f';
        const g = data.grade;
        if (g === 'A+' || g === 'A') gradeClass = 'grade-a';
        else if (g === 'B') gradeClass = 'grade-b';
        else if (g === 'C') gradeClass = 'grade-c';
        else if (g === 'D') gradeClass = 'grade-d';

        gradeDisplay.textContent = g;
        gradeDisplay.className = `grade-container ${gradeClass}`;

        // Header Pills
        headersPillsContainer.innerHTML = '';
        if (data.present_headers) {
            data.present_headers.forEach(h => {
                const span = document.createElement('span');
                span.className = 'pill pill-success';
                span.textContent = h.name;
                headersPillsContainer.appendChild(span);
            });
        }
        if (data.missing_headers) {
            data.missing_headers.forEach(h => {
                const span = document.createElement('span');
                span.className = 'pill pill-error';
                span.textContent = h.name;
                headersPillsContainer.appendChild(span);
            });
        }

        // Missing Headers
        const missingSection = document.getElementById('missing-headers-section');
        missingHeadersContainer.innerHTML = '';
        if (data.missing_headers && data.missing_headers.length > 0) {
            missingSection.classList.remove('hidden');
            data.missing_headers.forEach(h => {
                const item = document.createElement('div');
                item.className = 'header-item missing-item';
                item.innerHTML = `
                    <div>
                        <span class="header-name">${escapeHTML(h.name)}</span>
                        <span class="badge-missing">Missing</span>
                    </div>
                    <div class="header-desc">${escapeHTML(h.description)}</div>
                `;
                missingHeadersContainer.appendChild(item);
            });
        } else {
            missingSection.classList.add('hidden');
        }

        // Present Headers
        const presentSection = document.getElementById('present-headers-section');
        presentHeadersContainer.innerHTML = '';
        if (data.present_headers && data.present_headers.length > 0) {
            presentSection.classList.remove('hidden');
            data.present_headers.forEach(h => {
                let badgeHTML = '<span class="badge-present">Present</span>';
                let warningsHTML = '';

                if (h.warnings && h.warnings.length > 0) {
                    badgeHTML += ' <span class="badge-warning">Warning</span>';
                    const warningList = h.warnings.map(w => `<li class="warning-text">${escapeHTML(w)}</li>`).join('');
                    warningsHTML = `<ul class="warnings-list">${warningList}</ul>`;
                }

                const item = document.createElement('div');
                item.className = 'header-item present-item';
                item.innerHTML = `
                    <div>
                        <span class="header-name">${escapeHTML(h.name)}</span>
                        ${badgeHTML}
                    </div>
                    <div class="header-desc">${escapeHTML(h.description)}</div>
                    ${warningsHTML}
                    <div class="header-value">${escapeHTML(h.value)}</div>
                `;
                presentHeadersContainer.appendChild(item);
            });
        } else {
            presentSection.classList.add('hidden');
        }

        // Raw Headers
        rawHeadersContainer.innerHTML = '';
        if (data.raw_headers && data.raw_headers.length > 0) {
            data.raw_headers.forEach(pair => {
                if (pair.length === 2) {
                    const tr = document.createElement('tr');
                    tr.innerHTML = `
                        <td class="raw-th">${escapeHTML(pair[0])}</td>
                        <td class="raw-td">${escapeHTML(pair[1])}</td>
                    `;
                    rawHeadersContainer.appendChild(tr);
                }
            });
        }
    }

    function escapeHTML(str) {
        return str.replace(/[&<>'"]/g,
            tag => ({
                '&': '&amp;',
                '<': '&lt;',
                '>': '&gt;',
                "'": '&#39;',
                '"': '&quot;'
            }[tag] || tag)
        );
    }
});
