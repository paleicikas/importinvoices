var Admin = {
    init: function() {
        this.initColumnManager();
        this.initRipple();
        this.initExportSelection();
        this.initTooltips();
        this.initPopovers();
    },

    initTooltips: function() {
        var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'))
        tooltipTriggerList.map(function (tooltipTriggerEl) {
            return new bootstrap.Tooltip(tooltipTriggerEl)
        })
    },

    initPopovers: function() {
        var popoverTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="popover"]'))
        popoverTriggerList.map(function (popoverTriggerEl) {
            return new bootstrap.Popover(popoverTriggerEl)
        })
    },

    initRipple: function() {
        document.addEventListener('click', function(e) {
            const target = e.target.closest('.btn-primary, .btn-success, .btn-danger, .btn-warning, .quick-action-card');
            if (!target) return;

            const ripple = document.createElement('span');
            ripple.classList.add('ripple');
            target.appendChild(ripple);

            const rect = target.getBoundingClientRect();
            const size = Math.max(rect.width, rect.height);
            const x = e.clientX - rect.left - size / 2;
            const y = e.clientY - rect.top - size / 2;

            ripple.style.width = ripple.style.height = size + 'px';
            ripple.style.left = x + 'px';
            ripple.style.top = y + 'px';

            setTimeout(() => ripple.remove(), 600);
        });
    },

    applyColumnVisibility: function(table, hiddenCols) {
        if (!table) return;
        table.querySelectorAll('[data-col-index]').forEach(el => {
            const idx = el.dataset.colIndex;
            el.style.display = hiddenCols.includes(idx) ? 'none' : '';
        });
    },

    initColumnManager: function() {
        document.querySelectorAll('table[data-table-id]').forEach(table => {
            const tableId = table.dataset.tableId || 'default';
            const key = 'imp-cols-' + tableId;
            let hidden = [];
            try {
                hidden = JSON.parse(localStorage.getItem(key) || '[]');
            } catch (_) {}
            this.applyColumnVisibility(table, hidden);

            const picker = table.querySelector('.js-col-picker');
            if (!picker) return;

            picker.querySelectorAll('input[type=checkbox]').forEach(cb => {
                const idx = cb.dataset.colIndex;
                cb.checked = !hidden.includes(idx);
                cb.addEventListener('change', () => {
                    const hiddenCols = [...picker.querySelectorAll('input:not(:checked)')].map(x => x.dataset.colIndex);
                    localStorage.setItem(key, JSON.stringify(hiddenCols));
                    this.applyColumnVisibility(table, hiddenCols);
                });
            });
        });
    },

    initExportSelection: function() {
        const selectAll = document.getElementById('selectAllExport');
        const countSpan = document.getElementById('exportCount');
        if (!selectAll) return;
        
        const boxes = () => document.querySelectorAll('.export-invoice-id');
        
        const updateCount = () => {
            if (!countSpan) return;
            const checked = [...boxes()].filter(cb => cb.checked).length;
            countSpan.textContent = checked > 0 ? `(${checked})` : '';
        };

        selectAll.addEventListener('change', () => {
            boxes().forEach(cb => { cb.checked = selectAll.checked; });
            updateCount();
        });

        // Add listeners to individual checkboxes
        document.addEventListener('change', (e) => {
            if (e.target.classList.contains('export-invoice-id')) {
                updateCount();
                // Update selectAll state
                const all = boxes();
                const checked = [...all].filter(cb => cb.checked).length;
                selectAll.checked = checked === all.length;
                selectAll.indeterminate = checked > 0 && checked < all.length;
            }
        });

        if (boxes().length > 0) {
            selectAll.checked = true;
            boxes().forEach(cb => { cb.checked = true; });
            updateCount();
        }
        
        const form = document.getElementById('exportForm');
        const templateSelect = form && form.querySelector('[name="template_id"]');
        const formatSelect = form && form.querySelector('[name="format"]');
        if (templateSelect && formatSelect) {
            const sync = () => { formatSelect.disabled = templateSelect.value !== ''; };
            templateSelect.addEventListener('change', sync);
            sync();
        }
    }
};

document.addEventListener('DOMContentLoaded', () => {
    Admin.init();
});
