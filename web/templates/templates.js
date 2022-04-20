var templates = [
    { 'url': '^/netalert.*', 'tmpl': '/templates/netmap/records.html' },
    { 'url': '^/netmap.*', 'tmpl': '/templates/netmap/records.html' },
    { 'url': '^/alerts.*', 'tmpl': '/templates/alerts/alerts.html' },
    { 'url': '^/kubernetes.*', 'tmpl': '/templates/alerts/alerts.html' },
    { 'url': '^/monitoring.*', 'tmpl': '/templates/alerts/alerts.html' },
    { 'url': '^/vmetrics/targets', 'tmpl': '/templates/vmetrics/targets.html' },
    { 'url': '^/vmetrics/alerts', 'tmpl': '/templates/alerts/alerts.html' },
    { 'url': '^/prometheus/[^/]+/targets', 'tmpl': '/templates/vmetrics/targets.html' },
    { 'url': '^/prometheus/[^/]+/alerts', 'tmpl': '/templates/vmetrics/alerts.html' },
];