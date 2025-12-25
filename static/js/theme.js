// Theme Management System for Cold Storage App
// Mirrors the i18n pattern for consistency

const theme = {
    current: localStorage.getItem('theme') || 'light',

    init() {
        // Apply theme immediately to prevent flash
        this.apply(this.current);

        // Update toggle button icons
        this.updateIcons();
    },

    toggle() {
        this.current = this.current === 'light' ? 'dark' : 'light';
        localStorage.setItem('theme', this.current);
        this.apply(this.current);
        this.updateIcons();
    },

    apply(mode) {
        document.documentElement.setAttribute('data-theme', mode);
        if (mode === 'dark') {
            document.body.classList.add('dark-mode');
        } else {
            document.body.classList.remove('dark-mode');
        }
    },

    updateIcons() {
        const moonIcons = document.querySelectorAll('.theme-icon-moon');
        const sunIcons = document.querySelectorAll('.theme-icon-sun');

        if (this.current === 'dark') {
            moonIcons.forEach(el => el.classList.add('hidden'));
            sunIcons.forEach(el => el.classList.remove('hidden'));
        } else {
            moonIcons.forEach(el => el.classList.remove('hidden'));
            sunIcons.forEach(el => el.classList.add('hidden'));
        }
    }
};

// Initialize theme on DOM ready
document.addEventListener('DOMContentLoaded', () => theme.init());

// Also apply immediately for faster response (before DOMContentLoaded)
(function() {
    const savedTheme = localStorage.getItem('theme') || 'light';
    document.documentElement.setAttribute('data-theme', savedTheme);
})();
