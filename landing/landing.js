/* Landing page — 10-language inline i18n, JS only toggles visibility.
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

        /* ee → et (BCP47 / og:locale convention) */
        bcp47: function (lang) {
            return lang === 'ee' ? 'et' : lang;
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

        /* ── Language detection: ?lang= → localStorage → Accept-Language → en ── */

        detect: function () {
            var q = new URL(window.location.href).searchParams.get('lang');
            if (this.isValid(q && q.toLowerCase())) { return q.toLowerCase(); }

            try {
                var ls = localStorage.getItem('lang');
                if (this.isValid(ls)) { return ls; }
            } catch (e) {}

            var nav = navigator.languages || [navigator.language || ''];
            for (var i = 0; i < nav.length; i++) {
                var c = String(nav[i] || '').toLowerCase().slice(0, 2);
                if (this.isValid(c)) { return c; }
            }
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

            // Persist + URL (without reload)
            try { localStorage.setItem('lang', lang); } catch (e) {}
            if (!opts || !opts.skipUrl) {
                var u = new URL(window.location.href);
                u.searchParams.set('lang', lang);
                history.replaceState(null, '', u.toString());
            }

            this.drawInstall();
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
