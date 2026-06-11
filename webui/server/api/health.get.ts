import { healthz, readyz } from "../utils/mock";

export default defineEventHandler(() => ({ healthz, readyz }));
