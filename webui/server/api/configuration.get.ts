import { config_toml, snapshot } from "../utils/mock";

export default defineEventHandler(() => ({
  path: snapshot.config.path,
  toml: config_toml,
  last_reloaded: snapshot.config.last_reloaded,
  reload_signal: snapshot.config.reload_signal,
}));
