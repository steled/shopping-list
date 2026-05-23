(function () {
  'use strict';

  var list    = document.getElementById('items-list');
  var addForm = document.getElementById('add-form');

  /* ── Helpers ─────────────────────────────────────────────────────────── */

  async function api(method, path, body) {
    var opts = { method: method, headers: { 'Content-Type': 'application/json' } };
    if (body != null) opts.body = JSON.stringify(body);
    var r = await fetch(path, opts);
    if (!r.ok) throw new Error('HTTP ' + r.status);
    return r.status === 204 ? null : r.json();
  }

  function removeEmpty() {
    var e = document.getElementById('empty-hint');
    if (e) e.remove();
  }

  function maybeShowEmpty() {
    if (!list.querySelector('li.item')) {
      var li = document.createElement('li');
      li.id = 'empty-hint';
      li.className = 'empty-hint';
      li.textContent = 'Keine Artikel – füge deinen ersten hinzu!';
      list.appendChild(li);
    }
  }

  /* ── Build DOM item ───────────────────────────────────────────────────── */

  function makeItem(item) {
    var li = document.createElement('li');
    li.className = 'item' + (item.checked ? ' checked' : '');
    li.dataset.id = item.id;

    var handle = document.createElement('span');
    handle.className = 'drag-handle';
    handle.setAttribute('aria-hidden', 'true');
    handle.textContent = '⠿';

    var checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.className = 'item-checkbox';
    checkbox.setAttribute('aria-label', 'Gekauft');
    checkbox.checked = Boolean(item.checked);

    var nameSpan = document.createElement('span');
    nameSpan.className = 'item-name';
    nameSpan.textContent = item.name;

    var qty = document.createElement('input');
    qty.type = 'number';
    qty.className = 'item-qty';
    qty.value = item.quantity;
    qty.min = '1';
    qty.max = '999';
    qty.setAttribute('aria-label', 'Anzahl');

    var delBtn = document.createElement('button');
    delBtn.className = 'btn-icon btn-danger';
    delBtn.title = 'Löschen';
    delBtn.setAttribute('aria-label', 'Artikel löschen');
    delBtn.textContent = '🗑';

    li.appendChild(handle);
    li.appendChild(checkbox);
    li.appendChild(nameSpan);
    li.appendChild(qty);
    li.appendChild(delBtn);

    return li;
  }

  /* ── Inline name editing ──────────────────────────────────────────────── */

  function startEdit(li, span) {
    if (li.querySelector('.item-name-input')) return; // already editing
    var input = document.createElement('input');
    input.type = 'text';
    input.className = 'item-name-input';
    input.value = span.textContent;
    span.replaceWith(input);
    input.focus();
    input.select();

    function commit() {
      var val = input.value.trim() || span.textContent;
      var newSpan = document.createElement('span');
      newSpan.className = 'item-name';
      newSpan.textContent = val;
      input.replaceWith(newSpan);
      saveItem(li).catch(function (e) { console.error('save failed', e); });
    }
    input.addEventListener('blur', commit);
    input.addEventListener('keydown', function (e) {
      if (e.key === 'Enter')  { e.preventDefault(); input.blur(); }
      if (e.key === 'Escape') { input.value = span.textContent; input.blur(); }
    });
  }

  /* ── Persist ──────────────────────────────────────────────────────────── */

  function getState(li) {
    var nameEl = li.querySelector('.item-name') || li.querySelector('.item-name-input');
    return {
      id:       parseInt(li.dataset.id, 10),
      name:     nameEl ? nameEl.textContent || nameEl.value : '',
      quantity: parseInt(li.querySelector('.item-qty').value, 10) || 1,
      checked:  li.querySelector('.item-checkbox').checked,
    };
  }

  function saveItem(li) {
    var s = getState(li);
    return api('PUT', '/api/items/' + s.id, { name: s.name, quantity: s.quantity, checked: s.checked });
  }

  /* ── Event delegation ─────────────────────────────────────────────────── */

  list.addEventListener('click', async function (e) {
    var li = e.target.closest('li.item');
    if (!li) return;

    if (e.target.closest('.btn-danger')) {
      var id = parseInt(li.dataset.id, 10);
      try {
        await api('DELETE', '/api/items/' + id);
        li.remove();
        maybeShowEmpty();
        refreshInsertZones();
      } catch (err) { console.error('delete failed', err); }
      return;
    }

    if (e.target.classList.contains('item-name')) {
      startEdit(li, e.target);
    }
  });

  list.addEventListener('change', async function (e) {
    var li = e.target.closest('li.item');
    if (!li) return;

    if (e.target.classList.contains('item-checkbox')) {
      li.classList.toggle('checked', e.target.checked);
    }
    try {
      await saveItem(li);
    } catch (err) { console.error('save failed', err); }
  });

  /* ── Add item ─────────────────────────────────────────────────────────── */

  addForm.addEventListener('submit', async function (e) {
    e.preventDefault();
    var nameInput = document.getElementById('new-name');
    var qtyInput  = document.getElementById('new-qty');
    var name = nameInput.value.trim();
    var qty  = parseInt(qtyInput.value, 10) || 1;
    if (!name) return;
    try {
      var item = await api('POST', '/api/items', { name: name, quantity: qty });
      removeEmpty();
      list.appendChild(makeItem(item));
      refreshInsertZones();
      nameInput.value = '';
      qtyInput.value  = '1';
      nameInput.focus();
    } catch (err) { console.error('create failed', err); }
  });

  /* ── Drag & drop reorder ──────────────────────────────────────────────── */

  var sortable = null;

  function initSortable() {
    if (typeof Sortable === 'undefined') return;
    sortable = Sortable.create(list, {
      handle:      '.drag-handle',
      animation:   150,
      ghostClass:  'sortable-ghost',
      dragClass:   'sortable-drag',
      filter:      '.item-name-input, .insert-zone, .insert-zone *',
      onEnd: async function () {
        refreshInsertZones();
        var ids = Array.from(list.querySelectorAll('li.item')).map(function (li) {
          return parseInt(li.dataset.id, 10);
        });
        try {
          await api('PATCH', '/api/items/reorder', { ids: ids });
        } catch (err) { console.error('reorder failed', err); }
      },
    });
  }

  /* ── Insert between items ────────────────────────────────────────────── */

  function makeInsertZone(afterID) {
    var zone = document.createElement('li');
    zone.className = 'insert-zone';
    zone.dataset.afterId = afterID;
    zone.innerHTML = '<button class="insert-btn" title="Hier einfügen" aria-label="Artikel hier einfügen">＋</button>';
    return zone;
  }

  function refreshInsertZones() {
    list.querySelectorAll('.insert-zone').forEach(function (z) { z.remove(); });
    var items = list.querySelectorAll('li.item');
    if (items.length === 0) return;
    list.insertBefore(makeInsertZone(0), items[0]);
    items.forEach(function (li) {
      li.insertAdjacentElement('afterend', makeInsertZone(parseInt(li.dataset.id, 10)));
    });
  }

  function showInsertForm(zone) {
    list.querySelectorAll('.insert-zone.open').forEach(function (z) {
      z.classList.remove('open');
      z.innerHTML = '<button class="insert-btn" title="Hier einfügen" aria-label="Artikel hier einfügen">＋</button>';
    });

    zone.classList.add('open');
    zone.innerHTML =
      '<form class="insert-form">' +
      '<input type="text" class="insert-name" placeholder="Neuer Artikel…" required autocomplete="off" aria-label="Artikelname">' +
      '<input type="number" class="insert-qty" value="1" min="1" max="999" aria-label="Anzahl">' +
      '<button type="submit" class="btn btn-primary btn-sm">＋</button>' +
      '<button type="button" class="btn-icon btn-danger btn-cancel" title="Abbrechen" aria-label="Abbrechen">✕</button>' +
      '</form>';

    var nameInput = zone.querySelector('.insert-name');
    nameInput.focus();

    zone.querySelector('.insert-form').addEventListener('submit', async function (e) {
      e.preventDefault();
      var name = nameInput.value.trim();
      var qty  = parseInt(zone.querySelector('.insert-qty').value, 10) || 1;
      if (!name) return;
      var afterID = parseInt(zone.dataset.afterId, 10);
      try {
        var item = await api('POST', '/api/items', { name: name, quantity: qty, after_id: afterID });
        removeEmpty();
        var newLi = makeItem(item);
        zone.replaceWith(newLi);
        refreshInsertZones();
      } catch (err) { console.error('insert failed', err); }
    });

    zone.querySelector('.btn-cancel').addEventListener('click', function () {
      zone.classList.remove('open');
      zone.innerHTML = '<button class="insert-btn" title="Hier einfügen" aria-label="Artikel hier einfügen">＋</button>';
    });

    nameInput.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') {
        zone.classList.remove('open');
        zone.innerHTML = '<button class="insert-btn" title="Hier einfügen" aria-label="Artikel hier einfügen">＋</button>';
      }
    });
  }

  list.addEventListener('click', function (e) {
    var btn = e.target.closest('.insert-btn');
    if (!btn) return;
    var zone = btn.closest('.insert-zone');
    if (zone) showInsertForm(zone);
  });

  /* ── Filter ───────────────────────────────────────────────────────────── */

  var filterBtn = document.getElementById('filter-btn');
  var filterActive = localStorage.getItem('sl_filter') === '1';

  function applyFilter() {
    list.classList.toggle('filter-active', filterActive);
    filterBtn.classList.toggle('active', filterActive);
    filterBtn.setAttribute('aria-pressed', String(filterActive));
    if (sortable) {
      sortable.option('disabled', filterActive);
    }
  }

  filterBtn.addEventListener('click', function () {
    filterActive = !filterActive;
    localStorage.setItem('sl_filter', filterActive ? '1' : '0');
    applyFilter();
  });

  initSortable();
  applyFilter();
  refreshInsertZones();
})();
