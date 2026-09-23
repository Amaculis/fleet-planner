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
        meta: { title: "pages.timeline.title" },
        component: () => import("@/views/Timeline.vue"),
      },
      {
        path: "my/trips",
        name: "myTrips",
        meta: { title: "pages.myTrips.title" },
        component: () => import("@/views/MyTrips.vue"),
      },
      {
        path: "buses",
        name: "buses",
        meta: { title: "pages.buses.title" },
        component: () => import("@/views/buses/BusList.vue"),
      },
      {
        path: "buses/new",
        name: "busNew",
        meta: { title: "pages.buses.new" },
        component: () => import("@/views/buses/BusForm.vue"),
      },
      {
        path: "buses/:id/edit",
        name: "busEdit",
        meta: { title: "pages.buses.edit" },
        component: () => import("@/views/buses/BusForm.vue"),
      },
      {
        path: "drivers",
        name: "drivers",
        meta: { title: "pages.drivers.title" },
        component: () => import("@/views/drivers/DriverList.vue"),
      },
      {
        path: "drivers/new",
        name: "driverNew",
        meta: { title: "pages.drivers.new" },
        component: () => import("@/views/drivers/DriverForm.vue"),
      },
      {
        path: "drivers/:id/edit",
        name: "driverEdit",
        meta: { title: "pages.drivers.edit" },
        component: () => import("@/views/drivers/DriverForm.vue"),
      },
      {
        path: "trips",
        name: "trips",
        meta: { title: "pages.trips.title" },
        component: () => import("@/views/trips/TripList.vue"),
      },
      {
        path: "trips/new",
        name: "tripNew",
        meta: { title: "pages.trips.new" },
        component: () => import("@/views/trips/TripForm.vue"),
      },
      {
        path: "trips/:id",
        name: "tripDetail",
        meta: { title: "pages.trips.detail" },
        component: () => import("@/views/trips/TripDetail.vue"),
      },
      {
        path: "trips/:id/edit",
        name: "tripEdit",
        meta: { title: "pages.trips.edit" },
        component: () => import("@/views/trips/TripForm.vue"),
      },
      {
        path: "users",
        name: "users",
        meta: { title: "pages.users.title" },
        component: () => import("@/views/users/UserList.vue"),
      },
      {
        path: "users/new",
        name: "userNew",
        meta: { title: "pages.users.new" },
        component: () => import("@/views/users/UserForm.vue"),
      },
      {
        path: "accessibility",
        name: "accessibility",
        meta: { title: "pages.accessibility.title", anonymous: true },
        component: () => import("@/views/Accessibility.vue"),
      },
      {
        path: ":pathMatch(.*)*",
        name: "notFound",
        meta: { title: "pages.notFound.title", anonymous: true },
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
