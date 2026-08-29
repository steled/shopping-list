(function () {
  'use strict';

  var categoriesContainer = document.getElementById('categories-container');
  var addForm = document.getElementById('add-form');
  var addCategoryBtn = document.getElementById('add-category-btn');

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

  // Empty state applies only when there are neither categories nor
  // uncategorized items — an empty (but existing) category doesn't count.
  function maybeShowEmpty() {
    var hasCategory = !!categoriesContainer.querySelector('.category-block:not(.uncategorized-block)');
    var hasAnyItem = !!categoriesContainer.querySelector('li.item');
    if (hasCategory || hasAnyItem) return;
    if (document.getElementById('empty-hint')) return;
    var div = document.createElement('div');
    div.id = 'empty-hint';
    div.className = 'empty-hint';
    div.textContent = 'Keine Artikel – füge deinen ersten hinzu!';
    categoriesContainer.parentNode.insertBefore(div, categoriesContainer);
  }

  function updateCategoryCounts() {
    document.querySelectorAll('.category-block').forEach(function (block) {
      var listEl = block.querySelector('.items-list');
      var countEl = block.querySelector('.category-count');
      if (listEl && countEl) countEl.textContent = String(listEl.querySelectorAll('li.item').length);
    });
  }

  function updateUncategorizedVisibility() {
    var block = document.querySelector('.uncategorized-block');
    var list = document.querySelector('.items-list[data-category-id=""]');
    if (!block || !list) return;
    block.hidden = list.querySelectorAll('li.item').length === 0;
  }

  /* ── Build DOM item ───────────────────────────────────────────────────── */

  function makeUncategorizeButton() {
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'btn-icon uncategorize-btn';
    btn.title = 'Aus Abteilung entfernen';
    btn.setAttribute('aria-label', 'Artikel aus Abteilung entfernen');
    btn.textContent = '↩';
    return btn;
  }

  // Adds/removes the uncategorize button on an existing row to match
  // whichever .items-list it currently lives in (used after a drag moves it
  // across the category / "Ohne Kategorie" boundary).
  function syncUncategorizeButton(li) {
    var listEl = li.closest('.items-list');
    var inCategory = !!(listEl && listEl.dataset.categoryId);
    var existing = li.querySelector('.uncategorize-btn');
    if (inCategory && !existing) {
      li.insertBefore(makeUncategorizeButton(), li.querySelector('.btn-danger'));
    } else if (!inCategory && existing) {
      existing.remove();
    }
  }

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
    if (item.category_id != null) li.appendChild(makeUncategorizeButton());
    li.appendChild(delBtn);

    return li;
  }

  /* ── Inline name editing ──────────────────────────────────────────────── */

  function startEdit(li, span) {
    if (li.querySelector('.item-name-input')) return; // already editing
    var input = document.createElement('input');
    input.type = 'text';
    input.className = 'item-name-input';
    input.draggable = false; // sits inside a native-draggable <li>; without this, clicking to position the caret can be hijacked as a drag-start
    input.value = span.textContent;
    span.replaceWith(input);
    input.focus();

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

  function startCategoryEdit(block, span) {
    if (block.querySelector('.category-name-input')) return;
    var input = document.createElement('input');
    input.type = 'text';
    input.className = 'category-name-input';
    input.draggable = false; // sits inside a native-draggable .category-block
    input.value = span.textContent;
    span.replaceWith(input);
    input.focus();

    function commit() {
      var val = input.value.trim() || span.textContent;
      var newSpan = document.createElement('span');
      newSpan.className = 'category-name';
      newSpan.textContent = val;
      input.replaceWith(newSpan);
      if (val !== span.textContent) {
        var id = parseInt(block.dataset.categoryId, 10);
        api('PUT', '/api/categories/' + id, { name: val })
          .catch(function (e) { console.error('rename category failed', e); });
      }
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
    var listEl = li.closest('.items-list');
    var catAttr = listEl ? listEl.dataset.categoryId : '';
    return {
      id:         parseInt(li.dataset.id, 10),
      name:       nameEl ? nameEl.textContent || nameEl.value : '',
      quantity:   parseInt(li.querySelector('.item-qty').value, 10) || 1,
      checked:    li.querySelector('.item-checkbox').checked,
      categoryId: catAttr ? parseInt(catAttr, 10) : null,
    };
  }

  function saveItem(li) {
    var s = getState(li);
    return api('PUT', '/api/items/' + s.id,
      { name: s.name, quantity: s.quantity, checked: s.checked, category_id: s.categoryId });
  }

  /* ── Drag-to-expand a collapsed category ─────────────────────────────── */
  // Hovering a dragged item over a collapsed category's header for a moment
  // opens it, so it can be dropped straight in. Desktop drags fire native
  // "dragover" (regular mouse events are suppressed during a native drag);
  // touch drags go through SortableJS's own pointer-simulated fallback,
  // which leaves normal "pointermove" alone — both are wired to the same
  // hit-test so either input works.

  var HOVER_OPEN_DELAY = 350;
  var dragHoverEl = null;
  var dragHoverTimer = null;

  function clearDragHover() {
    if (dragHoverTimer) { clearTimeout(dragHoverTimer); dragHoverTimer = null; }
    if (dragHoverEl) { dragHoverEl.classList.remove('dwell-hover'); dragHoverEl = null; }
  }

  function handleDragPointer(x, y) {
    var el = document.elementFromPoint(x, y);
    var header = el && el.closest('.category-block.collapsed .category-header');
    if (header === dragHoverEl) return;
    clearDragHover();
    dragHoverEl = header || null;
    if (!dragHoverEl) return;
    dragHoverEl.classList.add('dwell-hover');
    var target = dragHoverEl;
    dragHoverTimer = setTimeout(function () {
      var block = target.closest('.category-block');
      target.classList.remove('dwell-hover');
      if (dragHoverEl === target) dragHoverEl = null;
      if (block) block.classList.remove('collapsed');
    }, HOVER_OPEN_DELAY);
  }

  function onDragOverWatch(e) { e.preventDefault(); handleDragPointer(e.clientX, e.clientY); }
  function onPointerMoveWatch(e) { handleDragPointer(e.clientX, e.clientY); }

  function startDragWatch() {
    document.addEventListener('dragover', onDragOverWatch, true);
    document.addEventListener('pointermove', onPointerMoveWatch, true);
  }

  function stopDragWatch() {
    document.removeEventListener('dragover', onDragOverWatch, true);
    document.removeEventListener('pointermove', onPointerMoveWatch, true);
    clearDragHover();
  }

  /* ── Event delegation ─────────────────────────────────────────────────── */

  categoriesContainer.addEventListener('click', async function (e) {
    var categoryDelete = e.target.closest('.category-delete');
    if (categoryDelete) {
      onCategoryDeleteClick(categoryDelete.closest('.category-block'));
      return;
    }

    var toggle = e.target.closest('.category-toggle');
    if (toggle) {
      var block = toggle.closest('.category-block');
      block.classList.toggle('collapsed');
      toggle.setAttribute('aria-expanded', String(!block.classList.contains('collapsed')));
      return;
    }

    if (e.target.classList.contains('category-name')) {
      startCategoryEdit(e.target.closest('.category-block'), e.target);
      return;
    }

    var addCatBtn = e.target.closest('.category-add-btn');
    if (addCatBtn) {
      var ul = addCatBtn.closest('.category-block').querySelector('.items-list');
      var zones = ul.querySelectorAll('.insert-zone');
      var lastZone = zones[zones.length - 1];
      if (lastZone) showInsertForm(lastZone);
      return;
    }

    var uncatBtn = e.target.closest('.uncategorize-btn');
    if (uncatBtn) {
      onUncategorizeClick(uncatBtn.closest('li.item'));
      return;
    }

    var insertBtn = e.target.closest('.insert-btn');
    if (insertBtn) {
      var izone = insertBtn.closest('.insert-zone');
      if (izone) showInsertForm(izone);
      return;
    }

    var li = e.target.closest('li.item');
    if (!li) return;

    if (e.target.closest('.btn-danger')) {
      var id = parseInt(li.dataset.id, 10);
      try {
        await api('DELETE', '/api/items/' + id);
        var listEl = li.closest('.items-list');
        li.remove();
        maybeShowEmpty();
        refreshInsertZones(listEl);
        updateCategoryCounts();
        updateUncategorizedVisibility();
      } catch (err) { console.error('delete failed', err); }
      return;
    }

    if (e.target.classList.contains('item-name')) {
      startEdit(li, e.target);
    }
  });

  categoriesContainer.addEventListener('change', async function (e) {
    var li = e.target.closest('li.item');
    if (!li) return;

    if (e.target.classList.contains('item-checkbox')) {
      li.classList.toggle('checked', e.target.checked);
    }
    try {
      await saveItem(li);
    } catch (err) { console.error('save failed', err); }
  });

  /* ── Uncategorize / delete a category ────────────────────────────────── */

  async function onUncategorizeClick(li) {
    if (!li) return;
    var noneList = document.querySelector('.items-list[data-category-id=""]');
    if (!noneList) return;
    var fromList = li.closest('.items-list');

    var noneBlock = document.querySelector('.uncategorized-block');
    if (noneBlock) noneBlock.hidden = false;
    noneList.appendChild(li);
    syncUncategorizeButton(li);

    try { await saveItem(li); } catch (err) { console.error('uncategorize failed', err); }

    var ids = Array.from(noneList.querySelectorAll('li.item')).map(function (el) {
      return parseInt(el.dataset.id, 10);
    });
    try {
      await api('PATCH', '/api/items/reorder', { category_id: null, ids: ids });
    } catch (err) { console.error('reorder failed', err); }

    refreshInsertZones(noneList);
    if (fromList) refreshInsertZones(fromList);
    updateCategoryCounts();
  }

  async function onCategoryDeleteClick(block) {
    if (!block) return;
    var id = parseInt(block.dataset.categoryId, 10);
    var name = block.querySelector('.category-name').textContent;
    if (!confirm('Abteilung "' + name + '" löschen? Die Artikel bleiben erhalten und wandern nach "Ohne Kategorie".')) {
      return;
    }
    try {
      await api('DELETE', '/api/categories/' + id);
    } catch (err) { console.error('delete category failed', err); return; }

    var noneList = document.querySelector('.items-list[data-category-id=""]');
    var noneBlock = document.querySelector('.uncategorized-block');
    var itemsList = block.querySelector('.items-list');
    var movedItems = Array.from(itemsList.querySelectorAll('li.item'));
    if (movedItems.length && noneBlock) noneBlock.hidden = false;
    movedItems.forEach(function (li) {
      noneList.appendChild(li);
      syncUncategorizeButton(li);
    });

    var idx = itemSortables.findIndex(function (s) { return s.el === itemsList; });
    if (idx !== -1) { itemSortables[idx].destroy(); itemSortables.splice(idx, 1); }
    block.remove();

    if (movedItems.length) {
      var ids = Array.from(noneList.querySelectorAll('li.item')).map(function (el) {
        return parseInt(el.dataset.id, 10);
      });
      try {
        await api('PATCH', '/api/items/reorder', { category_id: null, ids: ids });
      } catch (err) { console.error('reorder failed', err); }
    }

    refreshInsertZones(noneList);
    updateCategoryCounts();
    maybeShowEmpty();
  }

  /* ── Create a category ────────────────────────────────────────────────── */

  function buildCategoryBlock(category) {
    var section = document.createElement('section');
    section.className = 'category-block cat-color-' + (category.color_idx % 6);
    section.dataset.categoryId = category.id;

    var header = document.createElement('div');
    header.className = 'category-header';

    var handle = document.createElement('span');
    handle.className = 'group-handle';
    handle.setAttribute('aria-hidden', 'true');
    handle.textContent = '⠿';

    var toggle = document.createElement('button');
    toggle.type = 'button';
    toggle.className = 'category-toggle';
    toggle.setAttribute('aria-expanded', 'true');
    toggle.setAttribute('aria-label', 'Ein-/Ausklappen');
    toggle.textContent = '▾';

    var nameSpan = document.createElement('span');
    nameSpan.className = 'category-name';
    nameSpan.textContent = category.name;

    var count = document.createElement('span');
    count.className = 'category-count';
    count.textContent = '0';

    var del = document.createElement('button');
    del.type = 'button';
    del.className = 'btn-icon btn-danger category-delete';
    del.title = 'Abteilung löschen';
    del.setAttribute('aria-label', 'Abteilung löschen');
    del.textContent = '🗑';

    header.appendChild(handle);
    header.appendChild(toggle);
    header.appendChild(nameSpan);
    header.appendChild(count);
    header.appendChild(del);

    var ul = document.createElement('ul');
    ul.className = 'items-list';
    ul.dataset.categoryId = category.id;

    var addBtn = document.createElement('button');
    addBtn.type = 'button';
    addBtn.className = 'category-add-btn';
    addBtn.textContent = '＋ Artikel';

    section.appendChild(header);
    section.appendChild(ul);
    section.appendChild(addBtn);
    return section;
  }

  function showNewCategoryForm() {
    var existing = document.getElementById('new-category-form');
    if (existing) { existing.querySelector('input').focus(); return; }

    var form = document.createElement('form');
    form.id = 'new-category-form';
    form.className = 'new-category-form';
    form.innerHTML =
      '<input type="text" class="new-category-name" placeholder="Neue Abteilung…" required autocomplete="off" aria-label="Name der Abteilung">' +
      '<button type="submit" class="btn btn-primary btn-sm">＋</button>' +
      '<button type="button" class="btn-icon btn-danger btn-cancel" title="Abbrechen" aria-label="Abbrechen">✕</button>';

    var uncategorizedBlock = document.querySelector('.uncategorized-block');
    categoriesContainer.insertBefore(form, uncategorizedBlock);

    var nameInput = form.querySelector('.new-category-name');
    nameInput.focus();

    form.addEventListener('submit', async function (e) {
      e.preventDefault();
      var name = nameInput.value.trim();
      if (!name) return;
      try {
        var category = await api('POST', '/api/categories', { name: name });
        removeEmpty();
        var block = buildCategoryBlock(category);
        form.replaceWith(block);
        initSingleItemSortable(block.querySelector('.items-list'));
        refreshInsertZones(block.querySelector('.items-list'));
      } catch (err) { console.error('create category failed', err); }
    });

    form.querySelector('.btn-cancel').addEventListener('click', function () { form.remove(); });
    nameInput.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') form.remove();
    });
  }

  if (addCategoryBtn) {
    addCategoryBtn.addEventListener('click', showNewCategoryForm);
  }

  /* ── Add item (defaults to "Ohne Kategorie") ─────────────────────────── */

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
      var noneList = document.querySelector('.items-list[data-category-id=""]');
      var noneBlock = document.querySelector('.uncategorized-block');
      if (noneBlock) noneBlock.hidden = false;
      noneList.appendChild(makeItem(item));
      refreshInsertZones(noneList);
      updateCategoryCounts();
      nameInput.value = '';
      qtyInput.value  = '1';
      nameInput.focus();
    } catch (err) { console.error('create failed', err); }
  });

  /* ── Drag & drop reorder ──────────────────────────────────────────────── */

  var itemSortables = [];
  var categorySortable = null;

  async function onItemDragEnd(evt) {
    var li = evt.item;
    var toList = evt.to;
    var categoryId = toList.dataset.categoryId ? parseInt(toList.dataset.categoryId, 10) : null;
    var moved = evt.from !== evt.to;

    if (moved) {
      syncUncategorizeButton(li);
      try { await saveItem(li); } catch (err) { console.error('move failed', err); }
    }

    var ids = Array.from(toList.querySelectorAll('li.item')).map(function (el) {
      return parseInt(el.dataset.id, 10);
    });
    try {
      await api('PATCH', '/api/items/reorder', { category_id: categoryId, ids: ids });
    } catch (err) { console.error('reorder failed', err); }

    refreshInsertZones(toList);
    if (moved) refreshInsertZones(evt.from);
    updateCategoryCounts();
    updateUncategorizedVisibility();
  }

  function initSingleItemSortable(ul) {
    if (typeof Sortable === 'undefined') return null;
    var s = Sortable.create(ul, {
      group:       'items',
      handle:      '.drag-handle',
      animation:   150,
      ghostClass:  'sortable-ghost',
      dragClass:   'sortable-drag',
      filter:      '.item-name-input, .insert-zone, .insert-zone *',
      preventOnFilter: false,
      onStart:     startDragWatch,
      onEnd:       function (evt) { stopDragWatch(); onItemDragEnd(evt); },
    });
    itemSortables.push(s);
    return s;
  }

  function initItemSortables() {
    document.querySelectorAll('.items-list').forEach(initSingleItemSortable);
  }

  function initCategorySortable() {
    if (typeof Sortable === 'undefined') return;
    categorySortable = Sortable.create(categoriesContainer, {
      handle:      '.group-handle',
      animation:   150,
      ghostClass:  'sortable-ghost',
      dragClass:   'sortable-drag',
      onEnd: async function () {
        var ids = Array.from(categoriesContainer.querySelectorAll('.category-block:not(.uncategorized-block)'))
          .map(function (el) { return parseInt(el.dataset.categoryId, 10); });
        try {
          await api('PATCH', '/api/categories/reorder', { ids: ids });
        } catch (err) { console.error('category reorder failed', err); }
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

  function refreshInsertZones(listEl) {
    if (!listEl) return;
    listEl.querySelectorAll('.insert-zone').forEach(function (z) { z.remove(); });
    var items = listEl.querySelectorAll('li.item');
    var frontZone = makeInsertZone(0);
    if (items.length === 0) {
      listEl.appendChild(frontZone);
    } else {
      listEl.insertBefore(frontZone, items[0]);
    }
    items.forEach(function (li) {
      li.insertAdjacentElement('afterend', makeInsertZone(parseInt(li.dataset.id, 10)));
    });
  }

  function showInsertForm(zone) {
    document.querySelectorAll('.insert-zone.open').forEach(function (z) {
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
      var listEl = zone.closest('.items-list');
      var catAttr = listEl ? listEl.dataset.categoryId : '';
      var categoryId = catAttr ? parseInt(catAttr, 10) : null;
      try {
        var item = await api('POST', '/api/items',
          { name: name, quantity: qty, after_id: afterID, category_id: categoryId });
        removeEmpty();
        var newLi = makeItem(item);
        zone.replaceWith(newLi);
        refreshInsertZones(listEl);
        updateCategoryCounts();
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

  /* ── Filter ───────────────────────────────────────────────────────────── */

  var filterBtn = document.getElementById('filter-btn');
  var filterActive = localStorage.getItem('sl_filter') === '1';

  function applyFilter() {
    document.querySelectorAll('.items-list').forEach(function (ul) {
      ul.classList.toggle('filter-active', filterActive);
    });
    filterBtn.classList.toggle('active', filterActive);
    filterBtn.setAttribute('aria-pressed', String(filterActive));
    itemSortables.forEach(function (s) { s.option('disabled', filterActive); });
  }

  filterBtn.addEventListener('click', function () {
    filterActive = !filterActive;
    localStorage.setItem('sl_filter', filterActive ? '1' : '0');
    applyFilter();
  });

  initCategorySortable();
  initItemSortables();
  applyFilter();
  document.querySelectorAll('.items-list').forEach(refreshInsertZones);
})();
