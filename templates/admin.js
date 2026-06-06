const COPY_ICON = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
</svg>`;

const CHECK_ICON = `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <polyline points="20 6 9 17 4 12"></polyline>
</svg>`;

function updateActiveCount() {
    const active = document.querySelectorAll('.badge:not(.badge-expired)').length;
    document.getElementById('active-count').textContent = active;
}

function copyURL(btn, url) {
    navigator.clipboard.writeText(url).then(function () {
        btn.classList.add('copied');
        btn.innerHTML = CHECK_ICON + ' Copied!';

        setTimeout(function () {
            btn.classList.remove('copied');
            btn.innerHTML = COPY_ICON + ' Copy';
        }, 2000);
    });
}

document.addEventListener('DOMContentLoaded', function () {
    updateActiveCount();

    // refresh the page every 30s so new links show up
    setTimeout(function () { location.reload(); }, 30000);
});
