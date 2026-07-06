/* Landing page — 21-language inline i18n, JS only toggles visibility.
 * No AJAX: every language lives as its own <main class="lang-page"> block
 * in index.html. All language metadata (names, flags, SEO title/description,
 * og:locale) is read from data-* attributes in the DOM, not hardcoded here.
 * jQuery handles DOM work; a single Landing object owns all state + behaviour. */
(function ($) {
    'use strict';

    var Landing = {
        /* Populated from the DOM at init time:
         *   LANGS  — from .lang-menu li[data-lang]
         *   NAMES  — from .lang-menu li text
         *   FLAGS  — from .lang-menu li .fi class
         * OG_LOCALE + per-lang META are read on demand from the active
         * .lang-page data-* attributes (data-og-locale / data-title /
         * data-desc / data-og-title). */
        LANGS: [],
        NAMES: {},
        FLAGS: {},

        INSTALL: {
            windows: 'iwr -useb https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.ps1 | iex',
            unix: 'curl -fsSL https://raw.githubusercontent.com/paleicikas/importinvoices/main/installer/install.sh | bash'
        },

        currentLang: null,
        platform: null,

        /* ── Helpers ── */

        isValid: function (lang) {
            return !!lang && $.inArray(lang, this.LANGS) !== -1;
        },

        /* ee → et, zh → zh-Hans, pt-br → pt-BR (BCP47 / og:locale convention) */
        bcp47: function (lang) {
            var map = { 'ee': 'et', 'zh': 'zh-Hans', 'pt-br': 'pt-BR' };
            return map[lang] || lang;
        },

        /* Read SEO metadata for a language from its .lang-page data attrs. */
        metaFor: function (lang) {
            var $page = $('.lang-page[data-lang="' + lang + '"]');
            return {
                title:    $page.data('title')    || '',
                desc:     $page.data('desc')     || '',
                ogTitle:  $page.data('og-title') || '',
                ogLocale: $page.data('og-locale') || 'en_US'
            };
        },

        /* ── Language detection: ?lang= URL param → en (no localStorage/Accept-Language) ── */

        detect: function () {
            // ?lang= query param — set by Cloudflare Worker per-domain default
            // (e.g. saskaitosuvedimas.lt → ?lang=lt). If absent, default to EN
            // (per spec: "jei lang nėra, kraunam en" — no localStorage, no
            // Accept-Language guessing).
            var q = (new URL(window.location.href).searchParams.get('lang') || '').toLowerCase();
            if (this.isValid(q)) { return q; }
            return 'en';
        },

        /* ── Head metadata ── */

        setMeta: function (attr, key, val) {
            var $m = $('meta[' + attr + '="' + key + '"]');
            if (!$m.length) {
                $m = $('<meta>').attr(attr, key).appendTo('head');
            }
            $m.attr('content', val);
        },

        /* ── YouTube iframe: only the active page's iframe gets a src ── */

        loadIframe: function ($page) {
            var $iframe = $page.find('iframe[data-src]');
            if ($iframe.length && !$iframe.attr('src')) {
                $iframe.attr('src', $iframe.attr('data-src'));
            }
        },

        unloadIframe: function ($page) {
            $page.find('iframe[src]').removeAttr('src');
        },

        /* ── Apply a language: toggle pages + update SEO + persist ── */

        apply: function (lang, opts) {
            if (!this.isValid(lang)) { lang = 'en'; }
            this.currentLang = lang;

            var self = this;
            $('.lang-page').each(function () {
                var $page = $(this);
                if ($page.data('lang') === lang) {
                    $page.removeAttr('hidden');
                    self.loadIframe($page);
                } else {
                    $page.attr('hidden', '');
                    self.unloadIframe($page);
                }
            });

            // SEO: <html lang>, title, meta, OG/Twitter, JSON-LD inLanguage
            var meta = this.metaFor(lang);
            document.documentElement.lang = this.bcp47(lang);
            document.documentElement.dir = (lang === 'ar' || lang === 'he' || lang === 'fa') ? 'rtl' : 'ltr';
            document.title = meta.title;
            this.setMeta('name', 'description', meta.desc);
            this.setMeta('property', 'og:title', meta.ogTitle);
            this.setMeta('property', 'og:description', meta.desc);
            this.setMeta('property', 'og:locale', meta.ogLocale);
            this.setMeta('name', 'twitter:title', meta.ogTitle);
            this.setMeta('name', 'twitter:description', meta.desc);

            var $ld = $('#jsonld');
            if ($ld.length) {
                try {
                    var data = JSON.parse($ld.text());
                    data.inLanguage = this.bcp47(lang);
                    $ld.text(JSON.stringify(data));
                } catch (e) {}
            }

            // Toggle button + menu active state (names/flags come from the menu DOM)
            $('#langFlag').attr('class', 'fi ' + this.FLAGS[lang]);
            $('#langName').text(this.NAMES[lang]);
            $('.lang-menu li').removeClass('active')
                .filter('[data-lang="' + lang + '"]').addClass('active');

            // Update ?lang= query param (without reload). No localStorage — the
            // URL is the single source of truth so Worker/domain defaults apply
            // cleanly on every fresh visit.
            if (!opts || !opts.skipUrl) {
                var u = new URL(window.location.href);
                if (u.searchParams.get('lang') !== lang) {
                    u.searchParams.set('lang', lang);
                    history.replaceState(null, '', u.toString());
                }
            }

            this.drawInstall();
            this.checkRuModal();
        },

        /* ── RU Modal ── */

        checkRuModal: function () {
            if (this.currentLang === 'ru') {
                if (sessionStorage.getItem('ru-modal-passed') !== 'true') {
                    this.showRuModal();
                }
            } else {
                this.hideRuModal();
            }
        },

        showRuModal: function () {
            if ($('#ru-modal').length === 0) {
                var html = '<div id="ru-modal" style="display: flex; position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); backdrop-filter: blur(10px); -webkit-backdrop-filter: blur(10px); z-index: 99999; justify-content: center; align-items: center;">' +
                    '<div style="background: white; padding: 2rem; border-radius: 8px; text-align: center; max-width: 400px; width: 90%; box-shadow: 0 4px 12px rgba(0,0,0,0.15);">' +
                        '<h2 style="margin-top: 0; margin-bottom: 1.5rem; color: #333;">Слава Україні!</h2>' +
                        '<input type="text" id="ru-modal-input" class="form-control" style="margin-bottom: 1rem; width: 100%; padding: 0.5rem; border: 1px solid #ccc; border-radius: 4px; box-sizing: border-box;" placeholder="Ответ...">' +
                        '<div style="display: flex; gap: 0.5rem;">' +
                            '<button id="ru-modal-cancel-btn" class="btn btn-secondary" style="flex: 1; padding: 0.5rem; background: #6c757d; color: white; border: none; border-radius: 4px; cursor: pointer;">Отмена</button>' +
                            '<button id="ru-modal-btn" class="btn btn-primary" style="flex: 1; padding: 0.5rem; background: #0d6efd; color: white; border: none; border-radius: 4px; cursor: pointer;">Продолжить</button>' +
                        '</div>' +
                    '</div>' +
                '</div>';
                $('body').append(html);
                
                var checkAnswer = function() {
                    var val = $('#ru-modal-input').val().trim().toLowerCase();
                    if (val === 'gerojam slava' || val === 'herojam slava' || val === 'героям слава' || val === 'heroyam slava') {
                        sessionStorage.setItem('ru-modal-passed', 'true');
                        $('#ru-modal').hide();
                    } else {
                        $('#ru-modal-input').css('border-color', 'red');
                    }
                };
                
                $('#ru-modal-btn').on('click', checkAnswer);
                $('#ru-modal-cancel-btn').on('click', function() {
                    $('#ru-modal').hide();
                    Landing.apply('uk');
                });
                $('#ru-modal-input').on('keypress', function(e) {
                    if (e.key === 'Enter') checkAnswer();
                }).on('input', function() {
                    $(this).css('border-color', '#ccc');
                });
            }
            $('#ru-modal').css('display', 'flex');
            setTimeout(function() { $('#ru-modal-input').val('').css('border-color', '#ccc').focus(); }, 100);
        },

        hideRuModal: function () {
            $('#ru-modal').hide();
        },

        /* ── Install command switcher + clipboard ── */

        detectPlatform: function () {
            return navigator.userAgent.indexOf('Win') !== -1 ? 'windows' : 'unix';
        },

        drawInstall: function () {
            var cmd = this.INSTALL[this.platform];
            $('.lang-page:not([hidden]) .cmd').text(cmd);
            $('.lang-page:not([hidden]) .pill')
                .removeClass('on')
                .filter('[data-p="' + this.platform + '"]').addClass('on');
        },

        initInstall: function () {
            var self = this;

            $('.pill').on('click', function () {
                self.platform = $(this).data('p');
                self.drawInstall();
            });

            $('.copy-btn').on('click', function () {
                var $btn = $(this);
                var text = self.INSTALL[self.platform];
                var done = $btn.data('copied') || 'Copied';
                var reset = $btn.data('copy') || 'Copy';

                var flash = function () {
                    $btn.html('<i class="fas fa-check"></i><span>' + done + '</span>');
                    setTimeout(function () {
                        $btn.html('<i class="fas fa-copy"></i><span>' + reset + '</span>');
                    }, 1600);
                };

                var fallback = function () {
                    var ta = document.createElement('textarea');
                    ta.value = text;
                    document.body.appendChild(ta);
                    ta.select();
                    try { document.execCommand('copy'); } catch (e) {}
                    document.body.removeChild(ta);
                    flash();
                };

                if (navigator.clipboard && navigator.clipboard.writeText) {
                    navigator.clipboard.writeText(text).then(flash, fallback);
                } else {
                    fallback();
                }
            });
        },

        /* ── Language dropdown ── */

        initDropdown: function () {
            var self = this;
            var $nav = $('#langNav');
            var $toggle = $('#langToggle');

            $toggle.on('click', function (e) {
                e.stopPropagation();
                var open = $nav.toggleClass('open').hasClass('open');
                $toggle.attr('aria-expanded', open ? 'true' : 'false');
            });

            $('.lang-menu li').on('click', function () {
                self.apply($(this).data('lang'));
                $nav.removeClass('open');
                $toggle.attr('aria-expanded', 'false');
            });

            $(document).on('click', function (e) {
                if (!$nav.is(e.target) && $nav.has(e.target).length === 0) {
                    $nav.removeClass('open');
                    $toggle.attr('aria-expanded', 'false');
                }
            });

            $(document).on('keydown', function (e) {
                if (e.key === 'Escape') {
                    $nav.removeClass('open');
                    $toggle.attr('aria-expanded', 'false');
                }
            });
        },

        /* ── Build LANGS / NAMES / FLAGS from the .lang-menu DOM ── */

        buildLangIndex: function () {
            var self = this;
            $('.lang-menu li').each(function () {
                var $li = $(this);
                var lang = $li.data('lang');
                if (!lang) { return; }
                self.LANGS.push(lang);
                self.NAMES[lang] = $.trim($li.text());
                var flagClass = ($li.find('.fi').attr('class') || '').split(/\s+/).pop();
                self.FLAGS[lang] = flagClass || '';
            });
        },

        /* ── Boot ── */

        init: function () {
            this.buildLangIndex();
            this.platform = this.detectPlatform();
            this.initInstall();
            this.initDropdown();
            this.apply(this.detect(), { skipUrl: false });
        }
    };

    window.Landing = Landing;
})(jQuery);

$(document).ready(function () {
    window.Landing.init();
});
