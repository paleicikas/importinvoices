var Admin = {
    init: function() {
        this.initColumnManager();
        this.initRipple();
        this.initExportSelection();
        this.initReExportSelection();
        this.initTooltips();
        this.initPopovers();
        this.initMergeCompanyPicker();
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
        if (!form) return;

        const quickControls = document.getElementById('exportQuickControls');
        const templateControls = document.getElementById('exportTemplateControls');
        const formatSelect = form.querySelector('[name="format"]');
        const templateSelect = form.querySelector('[name="template_id"]');
        const modeInputs = form.querySelectorAll('[name="export_mode"]');
        const validationMsg = document.getElementById('exportValidationMsg');

        const showValidation = (message) => {
            if (!validationMsg) return;
            if (message) {
                validationMsg.textContent = message;
                validationMsg.classList.remove('d-none');
            } else {
                validationMsg.textContent = '';
                validationMsg.classList.add('d-none');
            }
        };

        const currentMode = () => {
            const checked = form.querySelector('[name="export_mode"]:checked');
            return checked ? checked.value : 'quick';
        };

        const syncExportMode = () => {
            const mode = currentMode();
            const isTemplate = mode === 'template';

            if (quickControls) {
                quickControls.classList.toggle('d-none', isTemplate);
            }
            if (templateControls) {
                templateControls.classList.toggle('d-none', !isTemplate);
            }
            if (formatSelect) {
                formatSelect.disabled = isTemplate;
            }
            if (templateSelect) {
                templateSelect.disabled = !isTemplate;
                if (!isTemplate) {
                    templateSelect.selectedIndex = 0;
                }
            }
            showValidation('');
        };

        modeInputs.forEach((input) => {
            input.addEventListener('change', syncExportMode);
        });
        syncExportMode();

        form.addEventListener('submit', (e) => {
            const checkedCount = [...boxes()].filter(cb => cb.checked).length;
            if (checkedCount === 0) {
                e.preventDefault();
                showValidation(form.dataset.msgSelectInvoice || 'Select at least one invoice');
                return;
            }

            if (currentMode() === 'template' && templateSelect && !templateSelect.value) {
                e.preventDefault();
                showValidation(form.dataset.msgSelectTemplate || 'Select an export template');
                return;
            }

            showValidation('');
        });
    },

    initReExportSelection: function() {
        const countSpan = document.getElementById('reExportCount');
        const boxes = () => document.querySelectorAll('.re-export-invoice-id');
        const updateCount = () => {
            if (!countSpan) return;
            const checked = [...boxes()].filter(cb => cb.checked).length;
            countSpan.textContent = checked > 0 ? `(${checked})` : '';
        };
        document.addEventListener('change', (e) => {
            if (e.target.classList.contains('re-export-invoice-id')) {
                updateCount();
            }
        });
        updateCount();
    },

    initMergeCompanyPicker: function() {
        const form = document.querySelector('form[data-merge-form]');
        if (!form) return;

        const searchInput = form.querySelector('[data-merge-search]');
        const resultsBox = form.querySelector('[data-merge-results]');
        const targetIdInput = form.querySelector('[data-merge-target-id]');
        const submitBtn = form.querySelector('[data-merge-submit]');
        const excludeId = form.dataset.exclude;
        const msgNoResults = form.dataset.msgNoresults || 'No matching companies found.';
        const msgSelect = form.dataset.msgSelect || 'Select a company to merge into';

        let debounceTimer = null;
        let currentRequestId = 0;
        let activeIndex = -1;

        const clearSelection = () => {
            targetIdInput.value = '';
            submitBtn.disabled = true;
        };

        const closeResults = () => {
            resultsBox.classList.add('d-none');
            resultsBox.innerHTML = '';
            activeIndex = -1;
        };

        const renderResults = (items) => {
            resultsBox.innerHTML = '';
            if (!items.length) {
                const empty = document.createElement('div');
                empty.className = 'list-group-item small text-secondary py-2';
                empty.textContent = msgNoResults;
                resultsBox.appendChild(empty);
                resultsBox.classList.remove('d-none');
                activeIndex = -1;
                return;
            }
            items.forEach((item, idx) => {
                const a = document.createElement('button');
                a.type = 'button';
                a.className = 'list-group-item list-group-item-action small py-2';
                a.dataset.id = item.id;
                a.dataset.title = item.title;
                a.dataset.idx = idx;
                const titleNode = document.createElement('span');
                titleNode.className = 'fw-bold';
                titleNode.textContent = item.title;
                a.appendChild(titleNode);
                if (item.vat_code) {
                    const vatNode = document.createElement('span');
                    vatNode.className = 'text-secondary ms-2 font-monospace';
                    vatNode.textContent = '(' + item.vat_code + ')';
                    a.appendChild(vatNode);
                }
                a.addEventListener('click', () => selectItem(item));
                resultsBox.appendChild(a);
            });
            resultsBox.classList.remove('d-none');
            activeIndex = -1;
        };

        const selectItem = (item) => {
            targetIdInput.value = item.id;
            searchInput.value = item.title;
            submitBtn.disabled = false;
            closeResults();
        };

        const performSearch = () => {
            const q = searchInput.value.trim();
            clearSelection();
            if (q.length < 1) {
                closeResults();
                return;
            }
            const reqId = ++currentRequestId;
            const url = '/api/v1/companies/search?q=' + encodeURIComponent(q) + '&exclude=' + encodeURIComponent(excludeId) + '&limit=20';
            fetch(url, { headers: { 'Accept': 'application/json' } })
                .then(resp => resp.ok ? resp.json() : [])
                .then(data => {
                    if (reqId !== currentRequestId) return;
                    renderResults(Array.isArray(data) ? data : []);
                })
                .catch(() => {
                    if (reqId !== currentRequestId) return;
                    closeResults();
                });
        };

        const highlightActive = () => {
            const items = resultsBox.querySelectorAll('button.list-group-item-action');
            items.forEach((el, i) => {
                el.classList.toggle('active', i === activeIndex);
            });
            if (activeIndex >= 0 && items[activeIndex]) {
                items[activeIndex].scrollIntoView({ block: 'nearest' });
            }
        };

        searchInput.addEventListener('input', () => {
            clearSelection();
            clearTimeout(debounceTimer);
            debounceTimer = setTimeout(performSearch, 200);
        });

        searchInput.addEventListener('focus', () => {
            if (searchInput.value.trim().length >= 1) performSearch();
        });

        searchInput.addEventListener('keydown', (e) => {
            const items = resultsBox.querySelectorAll('button.list-group-item-action');
            if (e.key === 'ArrowDown' && items.length) {
                e.preventDefault();
                activeIndex = Math.min(activeIndex + 1, items.length - 1);
                highlightActive();
            } else if (e.key === 'ArrowUp' && items.length) {
                e.preventDefault();
                activeIndex = Math.max(activeIndex - 1, 0);
                highlightActive();
            } else if (e.key === 'Enter') {
                if (activeIndex >= 0 && items[activeIndex]) {
                    e.preventDefault();
                    items[activeIndex].click();
                }
            } else if (e.key === 'Escape') {
                closeResults();
            }
        });

        document.addEventListener('click', (e) => {
            if (!form.contains(e.target)) closeResults();
        });

        form.addEventListener('submit', (e) => {
            if (!targetIdInput.value) {
                e.preventDefault();
                searchInput.focus();
            }
        });
    }
};

document.addEventListener('DOMContentLoaded', () => {
    Admin.init();
});
