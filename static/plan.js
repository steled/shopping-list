(function () {
  'use strict';

  var U = window.UI;
  var h = U.h;
  var root = document.getElementById('plan-root');
  var dialog = document.getElementById('plan-dialog');

  var params = new URLSearchParams(window.location.search);
  var checkDate = /^\d{4}-\d{2}-\d{2}$/.test(params.get('check') || '') ? params.get('check') : null;
  var month = /^\d{4}-\d{2}$/.test(params.get('month') || '') ? params.get('month')
    : checkDate ? checkDate.slice(0, 7) : null;

  var state = { plan: null, recipes: [], categories: [], items: [], confirmReroll: false };

  function recipeById(id) {
    for (var i = 0; i < state.recipes.length; i++) if (state.recipes[i].id === id) return state.recipes[i];
    return null;
  }
  function categoryById(id) {
    for (var i = 0; i < state.categories.length; i++) if (state.categories[i].id === id) return state.categories[i];
    return null;
  }
  function norm(s) { return s.trim().toLowerCase(); }
  function openItemNamed(name) {
    return state.items.some(function (it) { return !it.checked && norm(it.name) === norm(name); });
  }
  function shiftMonth(key, n) {
    var d = U.parseDate(key + '-01');
    d = new Date(d.getFullYear(), d.getMonth() + n, 1);
    return d.getFullYear() + '-' + U.pad(d.getMonth() + 1);
  }
  function isPast(date) { return date < state.plan.today; }

  async function load() {
    try {
      var res = await Promise.all([
        U.api('GET', '/api/plan' + (month ? '?month=' + encodeURIComponent(month) : '')),
        U.api('GET', '/api/recipes'),
        U.api('GET', '/api/categories'),
        U.api('GET', '/api/items'),
      ]);
      state.plan = res[0]; state.recipes = res[1]; state.categories = res[2]; state.items = res[3];
      month = state.plan.month;
      render();
    } catch (err) {
      console.error('load plan failed', err);
      root.replaceChildren(h('p', { class: 'banner banner-warn' }, 'Der Plan konnte nicht geladen werden. Bitte die Seite neu laden.'));
    }
  }

  function setMonth(key) {
    month = key;
    state.confirmReroll = false;
    var url = new URL(window.location.href);
    url.search = '?month=' + key;
    history.replaceState(null, '', url);
    load();
  }

  // run performs a change, then reloads the plan. Resolves to whether the change worked.
  async function run(fn) {
    var ok = true;
    try { await fn(); } catch (err) { ok = false; console.error(err); }
    await load();
    if (!ok) U.toast('Das hat nicht geklappt. Bitte erneut versuchen.');
    return ok;
  }

  /* ── Rendering ────────────────────────────────────────────────────────── */

  function upcomingDates(today) {
    var d = U.parseDate(today);
    while (d.getDay() !== 6 && d.getDay() !== 0) d = U.addDays(d, 1);
    var sat = d.getDay() === 6 ? d : U.addDays(d, -1);
    return [U.isoDate(sat), U.isoDate(U.addDays(sat, 1))];
  }

  function render() {
    var p = state.plan;
    var ym = p.month.split('-').map(Number);
    var todayMonth = p.today.slice(0, 7);
    var out = [];

    out.push(h('div', { class: 'month-nav' },
      h('button', { type: 'button', class: 'btn-icon', 'aria-label': 'Vorheriger Monat', onclick: function () { setMonth(shiftMonth(p.month, -1)); } }, U.icon('prev')),
      h('h2', {}, U.MONTHS[ym[1] - 1] + ' ' + ym[0]),
      h('button', { type: 'button', class: 'btn-icon', 'aria-label': 'Nächster Monat', disabled: p.month >= shiftMonth(todayMonth, 12),
        onclick: function () { setMonth(shiftMonth(p.month, 1)); } }, U.icon('next'))
    ));

    var counts = {};
    p.slots.forEach(function (s) { if (s.recipe_id) counts[s.recipe_id] = (counts[s.recipe_id] || 0) + 1; });
    var future = p.slots.filter(function (s) { return !isPast(s.date); });

    out.push(h('div', { class: 'month-meta' },
      h('span', { class: 'muted-text' }, p.slots.length + ' Termine · ' + p.active_recipes + ' aktive Gerichte'),
      future.length > 0 && p.active_recipes > 1 && h('button', { type: 'button', class: 'btn btn-ghost',
        onclick: function () { state.confirmReroll = true; render(); } }, U.icon('dice'), 'Kommende neu würfeln')
    ));

    if (p.active_recipes === 0) {
      out.push(h('div', { class: 'banner banner-warn', role: 'status' },
        h('p', {}, h('strong', {}, 'Noch keine Gerichte. '), 'Legt eure Rezepte an, dann verteilt die App sie zufällig auf die Wochenenden.'),
        h('a', { class: 'btn btn-primary', href: '/recipes' }, 'Rezepte anlegen')));
    } else if (future.length > 0 && p.slots.length > p.active_recipes) {
      var extra = p.slots.length - p.active_recipes;
      out.push(h('div', { class: 'banner banner-warn', role: 'status' },
        h('p', {}, h('strong', {}, p.slots.length + ' Termine, aber nur ' + p.active_recipes + ' aktive Gerichte. '),
          (extra === 1 ? '1 Gericht kommt' : extra + ' Gerichte kommen') + ' diesen Monat zweimal vor, mit möglichst großem Abstand.'),
        h('a', { class: 'btn btn-ghost', href: '/recipes' }, 'Gericht anlegen')));
    }

    if (state.confirmReroll) {
      out.push(h('div', { class: 'confirm-strip' },
        h('p', {}, 'Alle kommenden Termine in diesem Monat neu würfeln? Termine, deren Zutaten schon auf der Liste stehen, bleiben unverändert.'),
        h('button', { type: 'button', class: 'btn btn-primary', onclick: function () {
          state.confirmReroll = false;
          run(function () { return U.api('POST', '/api/plan/reroll', { month: p.month }); }).then(function (ok) { if (ok) U.toast('Kommende Termine neu gewürfelt'); });
        } }, 'Neu würfeln'),
        h('button', { type: 'button', class: 'btn btn-ghost', onclick: function () { state.confirmReroll = false; render(); } }, 'Abbrechen')));
    }

    var groups = [];
    p.slots.forEach(function (s) {
      var d = U.parseDate(s.date);
      var sat = d.getDay() === 6 ? d : U.addDays(d, -1);
      var key = U.isoDate(sat);
      var g = groups.length && groups[groups.length - 1].key === key ? groups[groups.length - 1] : null;
      if (!g) groups.push(g = { key: key, sat: sat, slots: [] });
      g.slots.push(s);
    });
    var upcoming = upcomingDates(p.today);
    groups.forEach(function (g) { out.push(renderWeekend(g, counts, upcoming)); });

    root.replaceChildren.apply(root, out);
  }

  function renderWeekend(g, counts, upcoming) {
    var sun = U.addDays(g.sat, 1);
    var past = g.slots.every(function (s) { return isPast(s.date); });
    var isNext = g.slots.some(function (s) { return upcoming.indexOf(s.date) !== -1; });
    var thu = U.addDays(g.sat, -2);
    var title = 'Wochenende ' + U.pad(g.sat.getDate()) + '.' + (g.sat.getMonth() === sun.getMonth() ? '' : U.pad(g.sat.getMonth() + 1) + '.') + '–' + U.short(sun);
    var pending = g.slots.filter(function (s) { return !isPast(s.date) && !s.transferred && recipeById(s.recipe_id); });

    return h('section', { class: 'weekend' + (isNext ? ' is-next' : '') + (past ? ' is-past' : ''), 'aria-label': title, 'data-weekend': g.key },
      h('div', { class: 'weekend-head' },
        h('h3', {}, title),
        isNext && h('span', { class: 'chip chip-accent' }, g.slots.some(function (s) { return s.date === state.plan.today; }) || U.isoDate(sun) === state.plan.today ? 'Dieses Wochenende' : 'Nächstes Wochenende'),
        past && h('span', { class: 'chip chip-muted' }, 'vorbei'),
        !past && h('span', { class: 'weekend-shop' }, U.isoDate(thu) < state.plan.today ? 'Nachkauf Fr/Sa' : 'Einkauf Do ' + U.short(thu))),
      g.slots.map(function (s) { return renderSlot(s, counts); }),
      pending.length > 0 && h('div', { class: 'weekend-foot' },
        h('button', { type: 'button', class: 'btn btn-primary', onclick: function () { quickTransfer(pending); } }, U.icon('cart'), 'Alles übernehmen'),
        h('button', { type: 'button', class: 'btn btn-ghost', onclick: function () { openTransfer(pending); } }, 'Auswählen…'))
    );
  }

  function renderSlot(s, counts) {
    var r = recipeById(s.recipe_id);
    var past = isPast(s.date);
    var d = U.parseDate(s.date);
    var flags = [];
    if (s.transferred) {
      flags.push(h('span', { class: 'chip chip-ok' }, s.open_items > 0
        ? '✓ ' + s.open_items + (s.open_items === 1 ? ' Zutat' : ' Zutaten') + ' auf der Liste'
        : '✓ übernommen'));
    }
    if (r && counts[r.id] > 1) flags.push(h('span', { class: 'chip chip-warn', title: 'Dieses Gericht kommt diesen Monat mehrfach vor' }, counts[r.id] + '× im Monat'));
    if (r && !r.active) flags.push(h('span', { class: 'chip chip-muted' }, 'pausiert'));

    var row = h('div', { class: 'slot' },
      h('div', { class: 'slot-day' }, h('b', {}, U.DOW[d.getDay()]), ' ' + U.short(d)),
      h('div', { class: 'slot-main' },
        h('button', { type: 'button', class: 'dish', disabled: past || state.recipes.length === 0,
          title: past ? null : 'Gericht selbst auswählen', onclick: function () { openPicker(s); } },
          r ? r.name : '– kein Gericht –', !past && h('span', { class: 'dish-caret', 'aria-hidden': 'true' }, '▾')),
        flags.length > 0 && h('div', { class: 'slot-flags' }, flags)),
      h('div', { class: 'slot-actions' },
        h('button', { type: 'button', class: 'btn-icon', disabled: past || state.plan.active_recipes < 2,
          title: 'Nur diesen Tag neu würfeln', 'aria-label': U.dayLabel(s.date) + ' neu würfeln',
          onclick: function () { run(function () { return U.api('POST', '/api/plan/' + s.date + '/reroll'); }); } }, U.icon('dice')),
        h('button', { type: 'button', class: 'btn-icon', disabled: past || !r,
          title: s.transferred ? 'Zutaten erneut prüfen' : 'Zutaten dieses Tages auswählen',
          'aria-label': 'Zutaten für ' + U.dayLabel(s.date) + ' auswählen', onclick: function () { openTransfer([s]); } }, U.icon('cart')))
    );

    if (s.stale_items > 0 && !past) {
      row.append(h('div', { class: 'slot-prompt', role: 'status' },
        h('p', {}, (s.stale_items === 1 ? '1 Zutat' : s.stale_items + ' Zutaten') + ' vom vorher geplanten Gericht ' + (s.stale_items === 1 ? 'steht' : 'stehen') + ' noch auf der Liste.'),
        h('button', { type: 'button', class: 'btn btn-danger-soft', onclick: function () {
          run(function () { return U.api('POST', '/api/plan/' + s.date + '/stale', { remove: true }); }).then(function (ok) { if (ok) U.toast('Von der Liste genommen'); });
        } }, 'Von der Liste nehmen'),
        h('button', { type: 'button', class: 'btn btn-ghost', onclick: function () {
          run(function () { return U.api('POST', '/api/plan/' + s.date + '/stale', { remove: false }); });
        } }, 'Behalten')));
    }
    return row;
  }

  /* ── Dialogs ──────────────────────────────────────────────────────────── */

  function showDialog(titleText, subText, body, foot) {
    dialog.replaceChildren(
      h('div', { class: 'sheet-head' },
        h('div', { class: 'sheet-title' }, h('h2', { id: 'plan-dialog-title' }, titleText), subText && h('p', {}, subText)),
        h('button', { type: 'button', class: 'btn-icon', 'aria-label': 'Schließen', onclick: function () { dialog.close(); } }, U.icon('close'))),
      body,
      foot && h('div', { class: 'sheet-foot' }, foot));
    if (!dialog.open) dialog.showModal();
  }

  dialog.addEventListener('click', function (e) { if (e.target === dialog) dialog.close(); });

  function openPicker(slot) {
    var usedOn = function (id) {
      return state.plan.slots.filter(function (s) { return s.date !== slot.date && s.recipe_id === id; })
        .map(function (s) { return U.dayLabel(s.date); });
    };
    var list = state.recipes.filter(function (r) { return r.active || r.id === slot.recipe_id; }).slice()
      .sort(function (a, b) { return usedOn(a.id).length - usedOn(b.id).length || a.name.localeCompare(b.name, 'de'); });

    showDialog('Gericht für ' + U.dayLabel(slot.date), 'Oben stehen die Gerichte, die diesen Monat noch nicht dran sind.',
      h('div', { class: 'sheet-body' }, h('ul', { class: 'pick-list' }, list.map(function (r) {
        var used = usedOn(r.id);
        var current = r.id === slot.recipe_id;
        return h('li', {}, h('button', { type: 'button', 'aria-current': current ? 'true' : null, onclick: function () {
          dialog.close();
          if (current) return;
          run(function () { return U.api('PUT', '/api/plan/' + slot.date, { recipe_id: r.id }); });
        } },
          h('span', {}, r.name),
          current ? h('span', { class: 'chip chip-accent' }, 'aktuell')
            : used.length ? h('span', { class: 'chip chip-warn' }, 'schon am ' + used.join(', '))
            : h('span', { class: 'chip chip-ok' }, 'noch frei')));
      }))));
    var first = dialog.querySelector('.pick-list button');
    if (first) first.focus();
  }

  // Rows for the ingredient dialog: "home" = ticked = not bought. An
  // ingredient already on the list is not ticked: transferring it links the
  // existing entry (and raises its quantity if needed) instead of adding a
  // duplicate, so the entry stays tied to this dish too.
  function buildGroups(slots) {
    return slots.map(function (s) {
      var r = recipeById(s.recipe_id);
      return {
        slot: s, recipe: r,
        rows: r.ingredients.map(function (g) {
          var onList = openItemNamed(g.name);
          return { g: g, onList: onList, home: g.pantry };
        }),
      };
    });
  }

  function entriesFrom(groups) {
    return groups.map(function (grp) {
      return { date: grp.slot.date, ingredient_ids: grp.rows.filter(function (r) { return !r.home; }).map(function (r) { return r.g.id; }) };
    });
  }

  function hint(g) {
    var parts = [];
    if (g.quantity > 1) parts.push(g.quantity + '×');
    if (g.amount) parts.push(g.amount);
    return parts.join(' ');
  }

  function stateChip(row) {
    if (row.home) return h('span', { class: 'chip chip-ok' }, row.g.pantry ? 'Grundvorrat' : 'zu Hause');
    if (row.onList) return h('span', { class: 'chip chip-muted' }, 'schon auf der Liste');
    var c = categoryById(row.g.category_id);
    return h('span', { class: 'chip chip-cat ' + (c ? 'cat-color-' + (c.color_idx % 6) : 'cat-none') }, c ? c.name : 'Ohne Kategorie');
  }

  async function submitTransfer(groups) {
    var undoable = groups.every(function (grp) { return !grp.slot.transferred; });
    var res = await U.api('POST', '/api/plan/transfer', { entries: entriesFrom(groups) });
    var n = res.items;
    var msg = n === 0 ? 'Alles da – als erledigt markiert' : (n === 1 ? '1 Zutat' : n + ' Zutaten') + ' auf der Liste';
    U.toast(msg, undoable ? function () {
      run(function () {
        return Promise.all(groups.map(function (grp) { return U.api('DELETE', '/api/plan/' + grp.slot.date + '/items'); }));
      });
    } : null);
  }

  function openTransfer(slots) {
    var groups = buildGroups(slots);
    var submit = h('button', { type: 'button', class: 'btn btn-primary' });
    // Only ingredients not on the list yet count as "new"; ones already there
    // are just linked to the dish.
    function update() {
      var fresh = 0, linked = 0;
      groups.forEach(function (grp) {
        grp.rows.forEach(function (r) { if (!r.home) { if (r.onList) linked++; else fresh++; } });
      });
      submit.textContent = fresh === 1 ? '1 Zutat auf die Liste setzen'
        : fresh > 1 ? fresh + ' Zutaten auf die Liste setzen'
        : linked > 0 ? 'Übernehmen – alles steht schon auf der Liste'
        : 'Alles da – als erledigt markieren';
    }
    submit.addEventListener('click', function () {
      dialog.close();
      run(function () { return submitTransfer(groups); });
    });

    var body = h('div', { class: 'sheet-body' },
      h('p', { class: 'muted-text' }, 'Hakt ab, was ihr schon zu Hause habt. Alles andere kommt auf die Liste.'),
      groups.map(function (grp) {
        return h('section', { class: 'dish-section' },
          h('h3', {}, grp.recipe.name, h('span', { class: 'muted-text' }, U.dayLabel(grp.slot.date))),
          grp.rows.length === 0
            ? h('p', { class: 'muted-text' }, 'Für dieses Gericht sind noch keine Zutaten hinterlegt.')
            : h('ul', { class: 'ing-list' }, grp.rows.map(function (row) {
              var cb = h('input', { type: 'checkbox', checked: row.home });
              var chip = h('span', { class: 'ing-state' }, stateChip(row));
              var label = h('label', { class: 'ing-pick' + (row.home ? ' is-home' : '') }, cb,
                h('span', { class: 'ing-name' }, row.g.name, hint(row.g) && h('span', { class: 'ing-hint' }, hint(row.g))), chip);
              cb.addEventListener('change', function () {
                row.home = cb.checked;
                label.classList.toggle('is-home', row.home);
                chip.replaceChildren(stateChip(row));
                update();
              });
              return h('li', {}, label);
            })));
      }));
    update();

    var single = groups.length === 1;
    showDialog(single ? groups[0].recipe.name : 'Zutaten fürs Wochenende',
      single ? U.dayLabel(groups[0].slot.date) + ' · Was habt ihr schon zu Hause?' : groups.map(function (g) { return g.recipe.name; }).join(' + '),
      body,
      [h('button', { type: 'button', class: 'btn btn-ghost', onclick: function () { dialog.close(); } }, 'Abbrechen'), submit]);
    submit.focus();
  }

  // Schnellweg: everything except pantry staples.
  function quickTransfer(slots) {
    run(function () { return submitTransfer(buildGroups(slots)); });
  }

  /* ── Start ────────────────────────────────────────────────────────────── */

  load().then(function () {
    if (!checkDate || !state.plan) return;
    var url = new URL(window.location.href);
    url.searchParams.delete('check');
    url.searchParams.set('month', state.plan.month);
    history.replaceState(null, '', url);
    var d = U.parseDate(checkDate);
    var sat = U.isoDate(d.getDay() === 6 ? d : U.addDays(d, -1));
    var card = root.querySelector('[data-weekend="' + sat + '"]');
    if (card) card.scrollIntoView({ block: 'center' });
    var pending = state.plan.slots.filter(function (s) {
      var sd = U.parseDate(s.date);
      var ssat = U.isoDate(sd.getDay() === 6 ? sd : U.addDays(sd, -1));
      return ssat === sat && !isPast(s.date) && !s.transferred && recipeById(s.recipe_id);
    });
    if (pending.length) openTransfer(pending);
  });
})();
