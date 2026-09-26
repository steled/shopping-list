// Shared helpers for the plan and recipes pages.
(function () {
  'use strict';

  // Static SVG markup only — never user data.
  var ICONS = {
    dice: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="3" y="3" width="18" height="18" rx="3"/><circle cx="8.5" cy="8.5" r="1.2" fill="currentColor"/><circle cx="15.5" cy="15.5" r="1.2" fill="currentColor"/><circle cx="12" cy="12" r="1.2" fill="currentColor"/></svg>',
    cart: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="9" cy="20" r="1.5"/><circle cx="18" cy="20" r="1.5"/><path d="M2 3h3l2.7 12.4a1.5 1.5 0 0 0 1.5 1.1h8.7a1.5 1.5 0 0 0 1.5-1.1L21 8H6.2"/></svg>',
    prev: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M15 18l-6-6 6-6"/></svg>',
    next: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 18l6-6-6-6"/></svg>',
    close: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" aria-hidden="true"><path d="M6 6l12 12M18 6L6 18"/></svg>',
  };

  function icon(name) {
    var span = document.createElement('span');
    span.className = 'icon';
    span.innerHTML = ICONS[name];
    return span;
  }

  // h builds an element. Children may be nodes, strings (inserted as text),
  // arrays, or null/false/undefined (skipped).
  function h(tag, attrs) {
    var el = document.createElement(tag);
    Object.keys(attrs || {}).forEach(function (k) {
      var v = attrs[k];
      if (v == null || v === false) return;
      if (k === 'class') el.className = v;
      else if (k.slice(0, 2) === 'on') el.addEventListener(k.slice(2), v);
      else if (k === 'checked' || k === 'disabled' || k === 'value') el[k] = v;
      else el.setAttribute(k, v === true ? '' : String(v));
    });
    (function append(kids) {
      kids.forEach(function (kid) {
        if (kid == null || kid === false) return;
        if (Array.isArray(kid)) append(kid);
        else el.append(kid);
      });
    })(Array.prototype.slice.call(arguments, 2));
    return el;
  }

  async function api(method, path, body) {
    var opts = { method: method, headers: { 'Content-Type': 'application/json' } };
    if (body != null) opts.body = JSON.stringify(body);
    var r = await fetch(path, opts);
    if (r.status === 401) { window.location.href = '/login'; throw new Error('unauthorized'); }
    if (!r.ok) throw new Error('HTTP ' + r.status);
    return r.status === 204 ? null : r.json();
  }

  var DOW = ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'];
  var MONTHS = ['Januar', 'Februar', 'März', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember'];

  function pad(n) { return String(n).padStart(2, '0'); }
  function parseDate(s) { var p = s.split('-').map(Number); return new Date(p[0], p[1] - 1, p[2] || 1); }
  function isoDate(d) { return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()); }
  function addDays(d, n) { return new Date(d.getFullYear(), d.getMonth(), d.getDate() + n); }
  function short(d) { return pad(d.getDate()) + '.' + pad(d.getMonth() + 1) + '.'; }
  function dayLabel(s) { var d = parseDate(s); return DOW[d.getDay()] + ' ' + short(d); }

  var toastTimer = null;
  function toast(msg, undo) {
    var host = document.getElementById('toast-host');
    if (!host) return;
    clearTimeout(toastTimer);
    var el = h('div', { class: 'toast', role: 'status' }, h('span', {}, msg),
      undo && h('button', { type: 'button', onclick: function () { host.replaceChildren(); undo(); } }, 'Rückgängig'));
    host.replaceChildren(el);
    toastTimer = setTimeout(function () { host.replaceChildren(); }, 6000);
  }

  window.UI = {
    icon: icon, h: h, api: api, toast: toast,
    DOW: DOW, MONTHS: MONTHS, pad: pad, parseDate: parseDate, isoDate: isoDate,
    addDays: addDays, short: short, dayLabel: dayLabel,
  };
})();
