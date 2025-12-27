/**
 * Hover Prefetch - Preload API data on mouse hover for instant page loads
 *
 * Usage: Add data-prefetch attribute to navigation elements
 * Example: <button data-prefetch="/item-search" onclick="navigate('/item-search')">
 */

(function() {
    'use strict';

    // Configuration
    const CONFIG = {
        CACHE_TTL: 30000,        // 30 seconds cache lifetime
        DEBOUNCE_MS: 100,        // Wait 100ms before prefetching (avoid accidental hovers)
        MAX_CACHE_SIZE: 20,      // Maximum cached endpoints
        ENABLED: true            // Kill switch
    };

    // Page to API endpoint mapping
    const PAGE_ENDPOINTS = {
        '/item-search': ['/api/entries', '/api/room-entries'],
        '/entry-room': ['/api/entries/unassigned', '/api/room-entries'],
        '/main-entry': ['/api/customers', '/api/entries/count'],
        '/room-config-1': ['/api/entries/unassigned'],
        '/room-form-2': ['/api/room-entries'],
        '/gate-pass-entry': ['/api/entries', '/api/room-entries'],
        '/unloading-tickets': ['/api/gate-passes/pending', '/api/gate-passes/approved'],
        '/rent-management': ['/api/customers', '/api/rent-payments'],
        '/events': ['/api/entry-events'],
        '/guard/dashboard': ['/api/guard/entries/pending'],
        '/employees': ['/api/users'],
        '/customer-edit': ['/api/customers'],
        '/admin/logs': ['/api/login-logs', '/api/edit-logs'],
        '/monitoring': ['/api/monitoring/dashboard'],
        '/infrastructure': ['/api/infrastructure/postgresql-pods', '/api/infrastructure/backend-pods']
    };

    // Cache storage
    const cache = new Map();
    const pendingFetches = new Map();

    // Get auth token
    function getToken() {
        try {
            const user = JSON.parse(localStorage.getItem('user') || '{}');
            return user.token || localStorage.getItem('token') || '';
        } catch {
            return '';
        }
    }

    // Check if cache entry is valid
    function isCacheValid(entry) {
        return entry && (Date.now() - entry.timestamp) < CONFIG.CACHE_TTL;
    }

    // Clean old cache entries
    function cleanCache() {
        if (cache.size > CONFIG.MAX_CACHE_SIZE) {
            const entries = Array.from(cache.entries());
            entries.sort((a, b) => a[1].timestamp - b[1].timestamp);
            const toDelete = entries.slice(0, entries.length - CONFIG.MAX_CACHE_SIZE);
            toDelete.forEach(([key]) => cache.delete(key));
        }
    }

    // Prefetch a single endpoint
    async function prefetchEndpoint(endpoint) {
        const token = getToken();
        if (!token) return null;

        // Check cache
        const cached = cache.get(endpoint);
        if (isCacheValid(cached)) {
            return cached.data;
        }

        // Check if already fetching
        if (pendingFetches.has(endpoint)) {
            return pendingFetches.get(endpoint);
        }

        // Start fetch
        const fetchPromise = fetch(endpoint, {
            headers: {
                'Authorization': `Bearer ${token}`,
                'X-Prefetch': 'true'  // Mark as prefetch for logging
            }
        })
        .then(res => {
            if (!res.ok) throw new Error(`HTTP ${res.status}`);
            return res.json();
        })
        .then(data => {
            cache.set(endpoint, { data, timestamp: Date.now() });
            pendingFetches.delete(endpoint);
            cleanCache();
            return data;
        })
        .catch(err => {
            pendingFetches.delete(endpoint);
            console.debug(`[Prefetch] Failed: ${endpoint}`, err.message);
            return null;
        });

        pendingFetches.set(endpoint, fetchPromise);
        return fetchPromise;
    }

    // Prefetch all endpoints for a page
    function prefetchPage(pagePath) {
        const endpoints = PAGE_ENDPOINTS[pagePath];
        if (!endpoints || !endpoints.length) return;

        console.debug(`[Prefetch] Preloading ${pagePath}:`, endpoints);
        endpoints.forEach(prefetchEndpoint);
    }

    // Get cached data (for use by page scripts)
    window.getPrefetchedData = function(endpoint) {
        const cached = cache.get(endpoint);
        if (isCacheValid(cached)) {
            console.debug(`[Prefetch] Cache hit: ${endpoint}`);
            return Promise.resolve(cached.data);
        }
        return null;
    };

    // Enhanced fetch that checks prefetch cache first
    window.fetchWithPrefetch = async function(endpoint, options = {}) {
        const cached = cache.get(endpoint);
        if (isCacheValid(cached)) {
            console.debug(`[Prefetch] Using cached: ${endpoint}`);
            return cached.data;
        }

        // Fall back to normal fetch
        const token = getToken();
        const res = await fetch(endpoint, {
            ...options,
            headers: {
                'Authorization': `Bearer ${token}`,
                ...options.headers
            }
        });
        const data = await res.json();

        // Cache the result
        cache.set(endpoint, { data, timestamp: Date.now() });
        return data;
    };

    // Setup hover listeners
    function setupPrefetch() {
        if (!CONFIG.ENABLED) return;

        // Skip on mobile/touch devices (no hover)
        if ('ontouchstart' in window || navigator.maxTouchPoints > 0) {
            console.debug('[Prefetch] Disabled on touch device');
            return;
        }

        let hoverTimeout = null;

        // Find all navigation buttons
        document.querySelectorAll('[onclick*="navigate"], [onclick*="location.href"], a[href]').forEach(el => {
            // Extract target path
            let targetPath = null;

            const onclick = el.getAttribute('onclick') || '';
            const href = el.getAttribute('href');

            if (onclick.includes('navigate(')) {
                const match = onclick.match(/navigate\(['"]([^'"]+)['"]\)/);
                if (match) targetPath = match[1];
            } else if (onclick.includes('location.href')) {
                const match = onclick.match(/location\.href\s*=\s*['"]([^'"]+)['"]/);
                if (match) targetPath = match[1];
            } else if (href && !href.startsWith('#') && !href.startsWith('javascript:')) {
                targetPath = href;
            }

            if (!targetPath || !PAGE_ENDPOINTS[targetPath]) return;

            // Add hover listener with debounce
            el.addEventListener('mouseenter', () => {
                hoverTimeout = setTimeout(() => {
                    prefetchPage(targetPath);
                }, CONFIG.DEBOUNCE_MS);
            });

            el.addEventListener('mouseleave', () => {
                if (hoverTimeout) {
                    clearTimeout(hoverTimeout);
                    hoverTimeout = null;
                }
            });
        });

        console.debug('[Prefetch] Initialized with', Object.keys(PAGE_ENDPOINTS).length, 'page mappings');
    }

    // Initialize on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', setupPrefetch);
    } else {
        setupPrefetch();
    }

    // Expose for debugging
    window._prefetchCache = cache;
    window._prefetchConfig = CONFIG;

})();

/**
 * Fullscreen Support
 * - PWA mode: App is always fullscreen when installed on home screen
 * - Browser mode: Auto-fullscreen on tablets/phones only
 * - Toggle button available for manual control
 */

// Check if running as installed PWA
function isPWA() {
    return window.matchMedia('(display-mode: fullscreen)').matches ||
           window.matchMedia('(display-mode: standalone)').matches ||
           window.navigator.standalone === true;
}

// Check if touch device (tablet/phone)
function isTouchDevice() {
    return 'ontouchstart' in window || navigator.maxTouchPoints > 0;
}

// Check if in fullscreen
function isFullscreen() {
    return !!(
        document.fullscreenElement ||
        document.webkitFullscreenElement ||
        document.mozFullScreenElement ||
        document.msFullscreenElement
    );
}

// Request fullscreen
function enterFullscreen() {
    var elem = document.documentElement;
    if (elem.requestFullscreen) {
        elem.requestFullscreen().catch(function() {});
    } else if (elem.webkitRequestFullscreen) {
        elem.webkitRequestFullscreen();
    } else if (elem.mozRequestFullScreen) {
        elem.mozRequestFullScreen();
    } else if (elem.msRequestFullscreen) {
        elem.msRequestFullscreen();
    }
}

// Exit fullscreen
function exitFullscreen() {
    if (document.exitFullscreen) {
        document.exitFullscreen().catch(function() {});
    } else if (document.webkitExitFullscreen) {
        document.webkitExitFullscreen();
    } else if (document.mozCancelFullScreen) {
        document.mozCancelFullScreen();
    } else if (document.msExitFullscreen) {
        document.msExitFullscreen();
    }
}

// Toggle fullscreen (button onclick)
function toggleFullscreen() {
    if (isFullscreen()) {
        exitFullscreen();
    } else {
        enterFullscreen();
    }
}

// Update icon
function updateFullscreenIcon() {
    var icon = document.getElementById('fullscreenIcon');
    if (icon) {
        icon.className = isFullscreen() ? 'bi bi-fullscreen-exit' : 'bi bi-fullscreen';
    }
}

document.addEventListener('fullscreenchange', updateFullscreenIcon);
document.addEventListener('webkitfullscreenchange', updateFullscreenIcon);

// Auto-fullscreen on tablets/phones (not desktop, not PWA)
(function() {
    if (isTouchDevice() && !isPWA()) {
        // Enter fullscreen on first interaction
        var done = false;
        var handler = function() {
            if (!done) {
                done = true;
                enterFullscreen();
            }
        };
        document.addEventListener('click', handler, { once: true });
        document.addEventListener('touchend', handler, { once: true });
    }
})();

// Register Service Worker for PWA
if ('serviceWorker' in navigator) {
    navigator.serviceWorker.register('/static/sw.js').catch(function() {});
}
