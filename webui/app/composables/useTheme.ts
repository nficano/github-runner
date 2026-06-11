type Theme = "day" | "night";

const STORAGE_KEY = "theater-theme";

export const useTheme = () => {
  const theme = useState<Theme>("theater-theme", () => "night");

  const apply = (next: Theme) => {
    theme.value = next;
    if (import.meta.client) {
      try { localStorage.setItem(STORAGE_KEY, next); } catch {}
    }
  };

  const toggle = () => apply(theme.value === "day" ? "night" : "day");

  if (import.meta.client) {
    onMounted(() => {
      try {
        const saved = localStorage.getItem(STORAGE_KEY) as Theme | null;
        if (saved === "day" || saved === "night") theme.value = saved;
      } catch {}
    });
  }

  return { theme, apply, toggle };
};
