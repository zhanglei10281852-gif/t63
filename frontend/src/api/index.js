import request from "../utils/request";

export const login = (data) => request.post("/auth/login", data);

export const getCurrentUser = () => request.get("/auth/me");

export const getAreas = () => request.get("/areas");

export const getRoadSections = () => request.get("/road-sections");

export const getWorkers = () => request.get("/workers");

export const getDashboard = () => request.get("/dashboard");

export const getWorkPlans = (params) => request.get("/work-plans", { params });

export const updatePlanWorker = (id, data) =>
  request.put(`/work-plans/${id}/worker`, data);

export const checkin = (data) => request.post("/checkin", data);

export const getCheckinRecords = (params) =>
  request.get("/checkin-records", { params });

export const getInspections = (params) =>
  request.get("/inspections", { params });

export const createInspection = (data) => request.post("/inspections", data);

export const getVehicles = (params) => request.get("/vehicles", { params });

export const getVehicleRecords = (params) =>
  request.get("/vehicles/records", { params });

export const getMaintenanceReminders = () =>
  request.get("/vehicles/maintenance-reminders");

export const startVehicle = (data) => request.post("/vehicles/start", data);

export const returnVehicle = (data) => request.post("/vehicles/return", data);

export const updateVehicleStatus = (id, data) =>
  request.put(`/vehicles/${id}/status`, data);

export const getComplaints = (params) => request.get("/complaints", { params });

export const createComplaint = (data) => request.post("/complaints", data);

export const handleComplaint = (id, data) =>
  request.put(`/complaints/${id}/handle`, data);

export const assignComplaint = (id, data) =>
  request.put(`/complaints/${id}/assign`, data);

export const getMonthlyAssessment = (params) =>
  request.get("/assessment/monthly", { params });
