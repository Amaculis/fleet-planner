// Demo-mode seed data (GitHub Pages build only — see mockApi.js). Generated relative
// to "today" at load time, not hardcoded dates, so the demo never looks stale no
// matter when someone opens the page. Mirrors the shapes internal/http/api_*.go
// actually serve (see those files) and roughly the same variety used to
// stress-test the real timeline during development: a full month of trips, some
// unassigned, some cancelled, a couple of multi-day tours.

function toIso(date) {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}
function addDays(iso, n) {
  const [y, m, d] = iso.split("-").map(Number);
  const dt = new Date(y, m - 1, d + n);
  return toIso(dt);
}
function ts(iso, hhmm) {
  return `${iso}T${hhmm}`;
}

const CITIES = ["Rīga", "Liepāja", "Daugavpils", "Jelgava", "Jūrmala", "Ventspils", "Rēzekne", "Valmiera", "Cēsis", "Ogre", "Sigulda", "Tukums"];
const SLOTS = [
  ["07:00", "09:30"],
  ["10:30", "13:00"],
  ["14:00", "17:00"],
  ["18:00", "21:00"],
];

let nextId = 1;
function id() {
  return nextId++;
}

export function buildFixtures() {
  const isoToday = toIso(new Date());

  const buses = [
    { id: id(), plate: "RA-2201", model: "Mercedes-Benz Tourismo", seats: 55, status: "active", insuranceExpiry: addDays(isoToday, 120), inspectionExpiry: addDays(isoToday, 45) },
    { id: id(), plate: "RA-2202", model: "Setra S517HD", seats: 50, status: "active", insuranceExpiry: addDays(isoToday, 200), inspectionExpiry: addDays(isoToday, 12) },
    { id: id(), plate: "RA-2203", model: "Volvo 9700", seats: 52, status: "active", insuranceExpiry: addDays(isoToday, 300), inspectionExpiry: addDays(isoToday, 90) },
    { id: id(), plate: "RA-2204", model: "MAN Lion's Coach", seats: 48, status: "active", insuranceExpiry: addDays(isoToday, 20), inspectionExpiry: addDays(isoToday, 200) },
    { id: id(), plate: "RA-2205", model: "Scania Touring", seats: 45, status: "maintenance", insuranceExpiry: addDays(isoToday, 150), inspectionExpiry: addDays(isoToday, 150) },
  ];

  const drivers = [
    { id: id(), fullName: "Jānis Bērziņš", phone: "+371 20001001", licenseNumber: "LV-100001", licenseExpiry: addDays(isoToday, 400), hourlyRate: "12.50", payType: "hourly", isActive: true, anonymized: false },
    { id: id(), fullName: "Anna Kalniņa", phone: "+371 20001002", licenseNumber: "LV-100002", licenseExpiry: addDays(isoToday, 25), hourlyRate: "13.00", payType: "hourly", isActive: true, anonymized: false },
    { id: id(), fullName: "Māris Ozoliņš", phone: "+371 20001003", licenseNumber: "LV-100003", licenseExpiry: addDays(isoToday, 500), hourlyRate: null, payType: null, isActive: true, anonymized: false },
    { id: id(), fullName: "Laura Krūmiņa", phone: "+371 20001004", licenseNumber: "LV-100004", licenseExpiry: addDays(isoToday, 600), hourlyRate: "14.25", payType: "hourly", isActive: true, anonymized: false },
    { id: id(), fullName: "Edgars Liepiņš", phone: null, licenseNumber: "LV-100005", licenseExpiry: addDays(isoToday, 700), hourlyRate: null, payType: null, isActive: false, anonymized: false },
  ];

  const trips = [];
  function pick(arr, i) {
    return arr[i % arr.length];
  }

  // ~5 weeks centered on today, so week/month view navigation has data in both
  // directions without needing to page far.
  for (let dayOffset = -10; dayOffset <= 25; dayOffset++) {
    const iso = addDays(isoToday, dayOffset);
    for (let b = 0; b < buses.length; b++) {
      const bus = buses[b];
      if (bus.status !== "active") continue;
      const driver = drivers[b % drivers.length];
      const tripsToday = 1 + ((dayOffset + b) % 3);
      for (let s = 0; s < tripsToday; s++) {
        const [startT, endT] = SLOTS[s % SLOTS.length];
        const origin = pick(CITIES, dayOffset + b + s + 100);
        let destination = pick(CITIES, dayOffset + b + s + 103);
        if (destination === origin) destination = pick(CITIES, dayOffset + b + s + 105);

        const roll = (dayOffset * 31 + b * 7 + s + 1000) % 15;
        const status = roll === 0 ? "cancelled" : dayOffset < 0 && roll % 4 === 0 ? "completed" : "planned";
        const paymentStatus = ["unpaid", "reserved", "advance_paid", "paid"][roll % 4];
        const trip = {
          id: id(),
          origin,
          destination,
          scheduledStart: ts(iso, startT),
          scheduledEnd: ts(iso, endT),
          status,
          paymentStatus,
          notes: null,
          assignment: null,
        };
        if (status !== "cancelled" && roll % 8 !== 1) {
          trip.assignment = { busId: bus.id, busPlate: bus.plate, driverId: driver.id, driverName: driver.fullName };
        }
        trips.push(trip);
      }
    }
  }

  // A couple of multi-day tours, same shape as the ones used to design the
  // timeline's spanning-bar rendering.
  trips.push({
    id: id(),
    origin: "Rīga",
    destination: "Tallinn (multi-day tour)",
    scheduledStart: ts(addDays(isoToday, 2), "08:00"),
    scheduledEnd: ts(addDays(isoToday, 5), "18:00"),
    status: "planned",
    paymentStatus: "advance_paid",
    notes: "Cross-border charter",
    assignment: { busId: buses[0].id, busPlate: buses[0].plate, driverId: drivers[0].id, driverName: drivers[0].fullName },
  });
  trips.push({
    id: id(),
    origin: "Rīga",
    destination: "Vilnius (multi-day tour)",
    scheduledStart: ts(addDays(isoToday, 12), "07:00"),
    scheduledEnd: ts(addDays(isoToday, 13), "20:00"),
    status: "planned",
    paymentStatus: "reserved",
    notes: null,
    assignment: { busId: buses[0].id, busPlate: buses[0].plate, driverId: drivers[0].id, driverName: drivers[0].fullName },
  });

  const users = [
    { id: id(), email: "admin@demo.example", role: "admin", driverId: null, driverName: null, isActive: true },
    { id: id(), email: "dispatcher@demo.example", role: "dispatcher", driverId: null, driverName: null, isActive: true },
    { id: id(), email: "driver@demo.example", role: "driver", driverId: drivers[0].id, driverName: drivers[0].fullName, isActive: true },
  ];

  return { buses, drivers, trips, users, isoToday };
}
