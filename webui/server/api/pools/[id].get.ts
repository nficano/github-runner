import { pools } from "../../utils/mock";

export default defineEventHandler((event) => {
  const id = getRouterParam(event, "id");
  const pool = pools.find(p => p.name === id);
  if (!pool) throw createError({ statusCode: 404, statusMessage: "pool not found" });
  return pool;
});
