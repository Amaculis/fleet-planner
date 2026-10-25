// meta.access mirrors the server's RequireRole groups (internal/http/server.go) — it
// only decides whether the 403 page is shown instead of the view; the server enforces
// it regardless. meta.breadcrumbs drives LxShell's back button: with a single crumb it
// links there, with several it falls back to router.back().
const planner = (rights) => rights.isPlanner();
const admin = (rights) => rights.isAdmin();
const driver = (rights) => rights.isDriver();

const crumb = (text, name) => ({ text, to: { name } });

const routes = [
  {
    path: "/",
    name: "home",
    component: () => import("@/layouts/MainLayout.vue"),
    meta: { title: "pages.home.title" },
    children: [
      {
        path: "",
        name: "dashboard",
        meta: { title: "pages.dashboard.title" },
        component: () => import("@/views/Dashboard.vue"),
      },
      {
        path: "timeline",
        name: "timeline",
        meta: { title: "pages.timeline.title", access: planner },
        component: () => import("@/views/Timeline.vue"),
      },
      {
        path: "my/trips",
        name: "myTrips",
        meta: { title: "pages.myTrips.title", access: driver },
        component: () => import("@/views/MyTrips.vue"),
      },
      {
        path: "buses",
        name: "buses",
        meta: { title: "pages.buses.title", access: planner },
        component: () => import("@/views/buses/BusList.vue"),
      },
      {
        path: "buses/new",
        name: "busNew",
        meta: { title: "pages.buses.new", access: admin, breadcrumbs: [crumb("pages.buses.title", "buses")] },
        component: () => import("@/views/buses/BusForm.vue"),
      },
      {
        path: "buses/:id/edit",
        name: "busEdit",
        meta: { title: "pages.buses.edit", access: admin, breadcrumbs: [crumb("pages.buses.title", "buses")] },
        component: () => import("@/views/buses/BusForm.vue"),
      },
      {
        path: "drivers",
        name: "drivers",
        meta: { title: "pages.drivers.title", access: planner },
        component: () => import("@/views/drivers/DriverList.vue"),
      },
      {
        path: "drivers/new",
        name: "driverNew",
        meta: { title: "pages.drivers.new", access: admin, breadcrumbs: [crumb("pages.drivers.title", "drivers")] },
        component: () => import("@/views/drivers/DriverForm.vue"),
      },
      {
        path: "drivers/:id/edit",
        name: "driverEdit",
        meta: { title: "pages.drivers.edit", access: admin, breadcrumbs: [crumb("pages.drivers.title", "drivers")] },
        component: () => import("@/views/drivers/DriverForm.vue"),
      },
      {
        path: "clients",
        name: "clients",
        meta: { title: "pages.clients.title", access: planner },
        component: () => import("@/views/clients/ClientList.vue"),
      },
      {
        path: "clients/new",
        name: "clientNew",
        meta: { title: "pages.clients.new", access: planner, breadcrumbs: [crumb("pages.clients.title", "clients")] },
        component: () => import("@/views/clients/ClientForm.vue"),
      },
      {
        path: "clients/:id/edit",
        name: "clientEdit",
        meta: { title: "pages.clients.edit", access: planner, breadcrumbs: [crumb("pages.clients.title", "clients")] },
        component: () => import("@/views/clients/ClientForm.vue"),
      },
      {
        path: "trips",
        name: "trips",
        meta: { title: "pages.trips.title", access: planner },
        component: () => import("@/views/trips/TripList.vue"),
      },
      {
        path: "trips/new",
        name: "tripNew",
        meta: { title: "pages.trips.new", access: planner, breadcrumbs: [crumb("pages.trips.title", "trips")] },
        component: () => import("@/views/trips/TripForm.vue"),
      },
      {
        path: "trips/:id",
        name: "tripDetail",
        meta: { title: "pages.trips.detail", access: planner, breadcrumbs: [crumb("pages.trips.title", "trips")] },
        component: () => import("@/views/trips/TripDetail.vue"),
      },
      {
        path: "trips/:id/edit",
        name: "tripEdit",
        // Two crumbs, so back returns to wherever the form was opened from (the list
        // or the trip's detail page). tripDetail reuses the current :id param.
        meta: {
          title: "pages.trips.edit",
          access: planner,
          breadcrumbs: [crumb("pages.trips.title", "trips"), crumb("pages.trips.detail", "tripDetail")],
        },
        component: () => import("@/views/trips/TripForm.vue"),
      },
      {
        path: "users",
        name: "users",
        meta: { title: "pages.users.title", access: admin },
        component: () => import("@/views/users/UserList.vue"),
      },
      {
        path: "users/new",
        name: "userNew",
        meta: { title: "pages.users.new", access: admin, breadcrumbs: [crumb("pages.users.title", "users")] },
        component: () => import("@/views/users/UserForm.vue"),
      },
      {
        path: "statistics",
        name: "statistics",
        meta: { title: "pages.statistics.title", access: admin },
        component: () => import("@/views/Statistics.vue"),
      },
      {
        path: "accessibility",
        name: "accessibility",
        meta: { title: "pages.accessibility.title", anonymous: true },
        component: () => import("@/views/Accessibility.vue"),
      },
      {
        path: "forbidden",
        name: "forbidden",
        // LxErrorPage shows its own title — the shell header would repeat it.
        meta: { title: "pages.forbidden.title", hidePageHeader: true },
        component: () => import("@/views/Forbidden.vue"),
      },
      {
        path: ":pathMatch(.*)*",
        name: "notFound",
        meta: { title: "pages.notFound.title", anonymous: true, hidePageHeader: true },
        component: () => import("@/views/NotFound.vue"),
      },
    ],
  },
  {
    path: "/login",
    name: "login",
    meta: { title: "login.title", anonymous: true, onlyAnonymous: true },
    component: () => import("@/views/Login.vue"),
  },
];

export default routes;
