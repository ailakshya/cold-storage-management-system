/**
 * Direct Connection Manager
 *
 * Routes ALL /api/* requests via the fastest available path:
 *   1. LAN  (http://192.168.1.134:8080)         — same network, fastest
 *   2. Direct HTTPS (direct.gurukripacoldstore.in) — bypasses Cloudflare proxy
 *   3. Fallback: relative URL (Cloudflare tunnel)  — always works
 *
 * Caches detection result in sessionStorage (instant on subsequent pages).
 * Patches window.fetch() so existing code needs zero changes for API calls.
 * Exposes directUrl() for img/video src attributes.
 */
(function() {
    'use strict';

    var LAN_HOST = 'http://192.168.1.134:8080';
    var DIRECT_HOST = 'https://direct.gurukripacoldstore.in';
    var CACHE_KEY = 'directBase';

    // Globals
    window.DIRECT_BASE = '';
    var _resolve;
    window.directReady = new Promise(function(resolve) { _resolve = resolve; });

    // Save original fetch before patching
    var _originalFetch = window.fetch.bind(window);

    /**
     * Returns DIRECT_BASE + path for use in img.src, video.src, etc.
     * Call after directReady resolves, or accept that it may return relative path.
     */
    window.directUrl = function(path) {
        if (!path) return path;
        // Already absolute URL — don't prefix
        if (path.startsWith('http://') || path.startsWith('https://')) return path;
        return window.DIRECT_BASE + path;
    };

    /**
     * Smart fetch: tries DIRECT_BASE first, falls back to Cloudflare on error.
     * Automatically injects Authorization header from localStorage token.
     */
    window.directFetch = function(path, options) {
        return window.directReady.then(function() {
            options = Object.assign({}, options);

            // Auto-inject auth token if not already present
            if (!options.headers) options.headers = {};
            if (options.headers instanceof Headers) {
                if (!options.headers.has('Authorization')) {
                    var t = localStorage.getItem('token');
                    if (t) options.headers.set('Authorization', 'Bearer ' + t);
                }
            } else {
                if (!options.headers['Authorization'] && !options.headers['authorization']) {
                    var t2 = localStorage.getItem('token');
                    if (t2) options.headers['Authorization'] = 'Bearer ' + t2;
                }
            }

            if (window.DIRECT_BASE) {
                return _originalFetch(window.DIRECT_BASE + path, options)
                    .then(function(resp) {
                        // If we get a network-level error response, still return it
                        // (4xx/5xx are valid responses, not network errors)
                        return resp;
                    })
                    .catch(function(err) {
                        console.warn('[DirectConnect] Direct failed, falling back to Cloudflare:', err.message);
                        // Clear cache so next page re-detects
                        sessionStorage.removeItem(CACHE_KEY);
                        window.DIRECT_BASE = '';
                        // Retry via Cloudflare (relative URL)
                        return _originalFetch(path, options);
                    });
            }

            return _originalFetch(path, options);
        });
    };

    /**
     * Patch window.fetch to automatically route /api/* calls via direct connection.
     * Non-API calls (CDN, Razorpay, etc.) pass through unchanged.
     */
    window.fetch = function(url, options) {
        // Only intercept string URLs starting with /api/
        if (typeof url === 'string' && url.startsWith('/api/')) {
            return window.directFetch(url, options);
        }
        return _originalFetch(url, options);
    };

    // Detection
    function detect() {
        var host = window.location.hostname;
        var isProduction = host.includes('gurukripacoldstore.in');
        var isLAN = host.startsWith('192.168.');

        // Check sessionStorage cache — instant, no network call
        var cached = sessionStorage.getItem(CACHE_KEY);
        if (cached !== null) {
            window.DIRECT_BASE = cached;
            console.log('[DirectConnect] Using cached:', cached || '(Cloudflare)');
            _resolve();
            return;
        }

        // Try LAN (only if on production domain or LAN subnet)
        var lanPromise = (isProduction || isLAN) ? tryEndpoint(LAN_HOST, 1500) : Promise.resolve(false);

        lanPromise.then(function(lanOk) {
            if (lanOk) {
                window.DIRECT_BASE = LAN_HOST;
                sessionStorage.setItem(CACHE_KEY, LAN_HOST);
                console.log('[DirectConnect] Using LAN:', LAN_HOST);
                _resolve();
                return;
            }

            // Try Direct HTTPS (only on production domain)
            var directPromise = isProduction ? tryEndpoint(DIRECT_HOST, 3000) : Promise.resolve(false);

            return directPromise.then(function(directOk) {
                if (directOk) {
                    window.DIRECT_BASE = DIRECT_HOST;
                    sessionStorage.setItem(CACHE_KEY, DIRECT_HOST);
                    console.log('[DirectConnect] Using Direct HTTPS:', DIRECT_HOST);
                } else {
                    window.DIRECT_BASE = '';
                    sessionStorage.setItem(CACHE_KEY, '');
                    console.log('[DirectConnect] Using Cloudflare tunnel (fallback)');
                }
                _resolve();
            });
        }).catch(function() {
            window.DIRECT_BASE = '';
            sessionStorage.setItem(CACHE_KEY, '');
            console.log('[DirectConnect] Detection error, using Cloudflare');
            _resolve();
        });
    }

    function tryEndpoint(host, timeout) {
        var ctrl = new AbortController();
        var timer = setTimeout(function() { ctrl.abort(); }, timeout);
        return _originalFetch(host + '/health', {
            signal: ctrl.signal,
            mode: 'cors',
            credentials: 'omit'
        }).then(function(r) {
            clearTimeout(timer);
            return r.ok;
        }).catch(function() {
            clearTimeout(timer);
            return false;
        });
    }

    detect();
})();
