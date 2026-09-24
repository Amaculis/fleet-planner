import { ref } from "vue";
import MockAdapter from "axios-mock-adapter";
import api from "@/api";
import { buildFixtures } from "@/demo/fixtures";

// Wires an in-memory fake backend onto the app's one shared axios instance (see
// api.js), so every service file (services/*.js) and store works completely
// unchanged — they just get answered by this instead of the real Go API. Only
// loaded when APP_CONFIG.demo is true (see main.js), so the real production
// build (the one the Go server embeds) never even fetches this module.

// Module-level (not inside installDemoApi) so DemoBanner.vue can import and drive
// it directly — it's a plain Vue ref, not a Pinia store, since it needs to exist
// and be readable before main.js has necessarily set one up. Seeded from
// sessionStorage because DemoBanner.vue's role switch does a hard reload (see its
// comment for why), which would otherwise silently reset the choice back to
// "admin" every time.
export const demoRole = ref(sessionStorage.getItem("demoRole") || "admin");

function matchId(url) {
  const m = url.match(/\/(\d+)(?:\/|$)/);
  return m ? Number(m[1]) : null;
}

function overlaps(aStart, aEnd, bStart, bEnd) {
  return aStart < bEnd && bStart < aEnd;
}

// Mirrors internal/http/forms.go's rangeParams: when the caller omits from/to
// (TripList.vue's bare getTrips(), with no args, is exactly this case) the real
// server defaults to "today through the next 7 days," not an empty/unbounded
// range — without this, a mocked bare /trips call always returned [] (comparing
// against `undefined` always fails), even though the app itself has trips today.
function defaultRange(from, to) {
  if (from && to) return { from, to };
  const now = new Date();
  const todayIso = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
  const start = from ?? todayIso;
  const [y, m, d] = start.split("-").map(Number);
  const end = to ?? new Date(y, m - 1, d + 7).toISOString().slice(0, 10);
  return { from: start, to: end };
}

// DemoBanner.vue's "reset demo data" button just does a hard reload (see its own
// comment) — this module re-executes from scratch on any reload, rebuilding
// fixtures for free, so there's no separate in-place reset function to maintain.
const state = buildFixtures();

// DemoBanner.vue calls this directly (alongside setting demoRole) to update the
// auth store's user in place, without a real network round trip.
export function demoUserForRole(role) {
  const byRole = { admin: state.users[0], dispatcher: state.users[1], driver: state.users[2] };
  return byRole[role];
}

export function installDemoApi() {
  function nextId(list) {
    return list.reduce((max, x) => Math.max(max, x.id), 0) + 1;
  }

  function findAssignmentConflict(busId, driverId, start, end, excludeTripId) {
    return state.trips.find((t) => {
      if (t.id === excludeTripId || !t.assignment || t.status === "cancelled") return false;
      if (t.assignment.busId !== busId && t.assignment.driverId !== driverId) return false;
      return overlaps(t.scheduledStart, t.scheduledEnd, start, end);
    });
  }

  const mock = new MockAdapter(api(), { delayResponse: 250 });

  // --- Auth ------------------------------------------------------------------
  mock.onGet("/auth/me").reply(() => [200, { authenticated: true, user: demoUserForRole(demoRole.value), csrfToken: "demo" }]);
  mock.onPost("/auth/login").reply(() => [200, { user: demoUserForRole(demoRole.value), csrfToken: "demo" }]);
  mock.onPost("/auth/logout").reply(() => [200, {}]);

  // --- Buses -------------------------------------------------------------------
  // config.params, not a query string on config.url: axios-mock-adapter intercepts
  // before axios serializes { params: {...} } onto the URL, and every service call
  // here (services/fleet.js, services/trips.js) passes filters that way.
  //
  // Every list-endpoint regex below is ^-anchored to match the full path, not just
  // its end — an unanchored /\/trips$/ matches "/trips" but also "/my/trips" (both
  // end in "/trips"), and since axios-mock-adapter tries handlers in registration
  // order, that silently swallowed every /my/trips request into the plain trips
  // list handler instead (which then filtered by from/to params that request never
  // sends, always returning []) — a driver's dashboard showed 0 trips no matter
  // what, confirmed by adding a temporary console.log inside the real /my/trips
  // handler and watching it simply never fire.
  mock.onGet(/^\/buses$/).reply((config) => {
    const assignableOnly = config.params?.assignable === "1";
    const buses = assignableOnly ? state.buses.filter((b) => b.status === "active") : state.buses;
    return [200, buses];
  });
  mock.onGet(/\/buses\/\d+$/).reply((config) => {
    const bus = state.buses.find((b) => b.id === matchId(config.url));
    return bus ? [200, bus] : [404, { error: "not found" }];
  });
  mock.onPost("/buses").reply((config) => {
    const body = JSON.parse(config.data);
    const bus = { id: nextId(state.buses), ...body };
    state.buses.push(bus);
    return [201, bus];
  });
  mock.onPut(/\/buses\/\d+$/).reply((config) => {
    const id = matchId(config.url);
    const idx = state.buses.findIndex((b) => b.id === id);
    if (idx === -1) return [404, { error: "not found" }];
    state.buses[idx] = { ...state.buses[idx], ...JSON.parse(config.data), id };
    return [200, state.buses[idx]];
  });
  mock.onDelete(/\/buses\/\d+$/).reply((config) => {
    const id = matchId(config.url);
    state.buses = state.buses.filter((b) => b.id !== id);
    return [204, {}];
  });

  // --- Drivers -------------------------------------------------------------
  mock.onGet(/^\/drivers$/).reply((config) => {
    const assignableOnly = config.params?.assignable === "1";
    const drivers = assignableOnly ? state.drivers.filter((d) => d.isActive && !d.anonymized) : state.drivers;
    return [200, drivers];
  });
  mock.onGet(/\/drivers\/\d+$/).reply((config) => {
    const driver = state.drivers.find((d) => d.id === matchId(config.url));
    return driver ? [200, driver] : [404, { error: "not found" }];
  });
  mock.onPost("/drivers").reply((config) => {
    const body = JSON.parse(config.data);
    const driver = { id: nextId(state.drivers), anonymized: false, ...body };
    state.drivers.push(driver);
    return [201, driver];
  });
  mock.onPut(/\/drivers\/\d+$/).reply((config) => {
    const id = matchId(config.url);
    const idx = state.drivers.findIndex((d) => d.id === id);
    if (idx === -1) return [404, { error: "not found" }];
    state.drivers[idx] = { ...state.drivers[idx], ...JSON.parse(config.data), id };
    return [200, state.drivers[idx]];
  });
  mock.onPost(/\/drivers\/\d+\/anonymize$/).reply((config) => {
    const id = matchId(config.url);
    const driver = state.drivers.find((d) => d.id === id);
    if (!driver) return [404, { error: "not found" }];
    driver.fullName = `Erased driver #${driver.id}`;
    driver.phone = null;
    driver.licenseNumber = null;
    driver.isActive = false;
    driver.anonymized = true;
    return [200, driver];
  });

  // --- Trips -----------------------------------------------------------------
  mock.onGet(/^\/trips$/).reply((config) => {
    const { from, to } = defaultRange(config.params?.from, config.params?.to);
    const trips = state.trips.filter((t) => overlaps(t.scheduledStart, t.scheduledEnd, from, to));
    return [200, trips];
  });
  mock.onGet(/\/trips\/\d+$/).reply((config) => {
    const trip = state.trips.find((t) => t.id === matchId(config.url));
    return trip ? [200, trip] : [404, { error: "not found" }];
  });
  mock.onPost("/trips").reply((config) => {
    const body = JSON.parse(config.data);
    const trip = { id: nextId(state.trips), status: "planned", assignment: null, ...body };
    state.trips.push(trip);
    return [201, trip];
  });
  mock.onPut(/\/trips\/\d+$/).reply((config) => {
    const id = matchId(config.url);
    const idx = state.trips.findIndex((t) => t.id === id);
    if (idx === -1) return [404, { error: "not found" }];
    state.trips[idx] = { ...state.trips[idx], ...JSON.parse(config.data), id };
    return [200, state.trips[idx]];
  });
  mock.onDelete(/\/trips\/\d+$/).reply((config) => {
    const id = matchId(config.url);
    state.trips = state.trips.filter((t) => t.id !== id);
    return [204, {}];
  });
  mock.onPost(/\/trips\/\d+\/status$/).reply((config) => {
    const id = matchId(config.url);
    const trip = state.trips.find((t) => t.id === id);
    if (!trip) return [404, { error: "not found" }];
    trip.status = JSON.parse(config.data).status;
    return [200, trip];
  });
  mock.onPost(/\/trips\/\d+\/assign$/).reply((config) => {
    const id = matchId(config.url);
    const trip = state.trips.find((t) => t.id === id);
    if (!trip) return [404, { error: "not found" }];
    const { busId, driverId } = JSON.parse(config.data);
    const conflict = findAssignmentConflict(busId, driverId, trip.scheduledStart, trip.scheduledEnd, trip.id);
    if (conflict) {
      return [409, { error: `Bus or driver already booked on trip #${conflict.id} at an overlapping time.` }];
    }
    const bus = state.buses.find((b) => b.id === busId);
    const driver = state.drivers.find((d) => d.id === driverId);
    trip.assignment = { busId, busPlate: bus?.plate ?? "", driverId, driverName: driver?.fullName ?? "" };
    return [200, trip];
  });
  mock.onPost(/\/trips\/\d+\/unassign$/).reply((config) => {
    const id = matchId(config.url);
    const trip = state.trips.find((t) => t.id === id);
    if (!trip) return [404, { error: "not found" }];
    trip.assignment = null;
    return [200, trip];
  });

  // --- Users -----------------------------------------------------------------
  mock.onGet("/users").reply(() => [200, state.users]);
  mock.onPost("/users").reply((config) => {
    const body = JSON.parse(config.data);
    const driver = state.drivers.find((d) => d.id === body.driverId);
    const user = {
      id: nextId(state.users),
      email: body.email,
      role: body.role,
      driverId: body.driverId ?? null,
      driverName: driver?.fullName ?? null,
      isActive: true,
    };
    state.users.push(user);
    return [201, user];
  });
  mock.onPost(/\/users\/\d+\/activate$/).reply((config) => {
    const user = state.users.find((u) => u.id === matchId(config.url));
    if (!user) return [404, { error: "not found" }];
    user.isActive = true;
    return [200, user];
  });
  mock.onPost(/\/users\/\d+\/deactivate$/).reply((config) => {
    const user = state.users.find((u) => u.id === matchId(config.url));
    if (!user) return [404, { error: "not found" }];
    user.isActive = false;
    return [200, user];
  });
  mock.onPost(/\/users\/\d+\/password$/).reply(() => [200, {}]);

  // --- Driver's own trips ---------------------------------------------------
  mock.onGet("/my/trips").reply(() => {
    const driverId = demoUserForRole(demoRole.value).driverId;
    const trips = state.trips
      .filter((t) => t.assignment?.driverId === driverId)
      .map((t) => ({
        id: t.id,
        origin: t.origin,
        destination: t.destination,
        scheduledStart: t.scheduledStart,
        scheduledEnd: t.scheduledEnd,
        status: t.status,
        busPlate: t.assignment.busPlate,
      }));
    return [200, trips];
  });
  mock.onPost(/\/my\/trips\/\d+\/start$/).reply((config) => {
    const trip = state.trips.find((t) => t.id === matchId(config.url));
    if (!trip) return [404, { error: "not found" }];
    trip.status = "in_progress";
    return [200, trip];
  });
  mock.onPost(/\/my\/trips\/\d+\/finish$/).reply((config) => {
    const trip = state.trips.find((t) => t.id === matchId(config.url));
    if (!trip) return [404, { error: "not found" }];
    trip.status = "completed";
    return [200, trip];
  });
}
