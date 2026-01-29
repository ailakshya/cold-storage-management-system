function getHomeDashboard() {
    try {
        var user = JSON.parse(localStorage.getItem('user'));
        if (!user) return '/login';
        switch (user.role) {
            case 'admin': return '/admin/dashboard';
            case 'guard': return '/guard/dashboard';
            case 'accountant': return '/accountant/dashboard';
            default: return '/dashboard';
        }
    } catch (e) {
        return '/dashboard';
    }
}
function goHome() { window.location.href = getHomeDashboard(); }
