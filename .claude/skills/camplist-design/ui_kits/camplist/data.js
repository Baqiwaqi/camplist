window.camplistSeed = {
  user: 'Sam Rivera',
  lists: [
    { id: 'l1', name: 'Weekend camping', description: 'Two nights, car access, no showers', items: [
      { id: 'i1', name: 'Tent', category: 'Shelter' }, { id: 'i2', name: 'Sleeping bag', category: 'Sleep' }, { id: 'i3', name: 'Sleeping pad', category: 'Sleep' },
      { id: 'i4', name: 'Headlamp', category: 'Light' }, { id: 'i5', name: 'Stove + gas', category: 'Kitchen' }, { id: 'i6', name: 'Water filter', category: 'Kitchen' }, { id: 'i7', name: 'Rain jacket', category: 'Clothing' } ] },
    { id: 'l2', name: 'Day hike', description: 'Light pack, back before dark', items: [ { id: 'i8', name: 'Daypack', category: 'Bags' }, { id: 'i9', name: 'Snacks', category: 'Food' }, { id: 'i10', name: 'First aid kit', category: 'Safety' } ] },
  ],
  sessions: [
    { id: 's1', listId: 'l1', createdAt: 'Sep 5, 2026', checked: ['i1', 'i2', 'i4'] },
    { id: 's2', listId: 'l2', createdAt: 'Aug 22, 2026', checked: ['i8', 'i9', 'i10'] },
  ],
};