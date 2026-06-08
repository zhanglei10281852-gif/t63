import { createRouter, createWebHistory } from "vue-router";

const routes = [
  {
    path: "/login",
    name: "Login",
    component: () => import("../views/Login.vue"),
  },
  {
    path: "/",
    component: () => import("../views/Layout.vue"),
    redirect: "/dashboard",
    children: [
      {
        path: "dashboard",
        name: "Dashboard",
        component: () => import("../views/Dashboard.vue"),
        meta: { title: "仪表盘" },
      },
      {
        path: "work-monitor",
        name: "WorkMonitor",
        component: () => import("../views/WorkMonitor.vue"),
        meta: { title: "作业监控" },
      },
      {
        path: "quality-inspection",
        name: "QualityInspection",
        component: () => import("../views/QualityInspection.vue"),
        meta: { title: "质量抽查" },
      },
      {
        path: "assessment",
        name: "Assessment",
        component: () => import("../views/Assessment.vue"),
        meta: { title: "考核报表" },
      },
      {
        path: "vehicle",
        name: "Vehicle",
        component: () => import("../views/Vehicle.vue"),
        meta: { title: "车辆管理" },
      },
      {
        path: "complaint",
        name: "Complaint",
        component: () => import("../views/Complaint.vue"),
        meta: { title: "投诉工单" },
      },
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
});

router.beforeEach((to, from, next) => {
  const token = localStorage.getItem("token");
  if (to.path !== "/login" && !token) {
    next("/login");
  } else if (to.path === "/login" && token) {
    next("/");
  } else {
    next();
  }
});

export default router;
