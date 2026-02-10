import { SSHLogin } from "./components/ssh-login";
import { SettingsContext, type Theme } from "./settings-context";
import { type Language, getTranslations } from "./i18n";
import { useState, useEffect, useCallback } from "react";
import { LoadSettings } from "../../wailsjs/go/main/App";

export default function App() {
  const [theme, setThemeState] = useState<Theme>("light");
  const [language, setLanguageState] = useState<Language>("zh");

  // Load settings on mount
  useEffect(() => {
    LoadSettings().then((s) => {
      if (s.theme === "dark" || s.theme === "light") setThemeState(s.theme as Theme);
      if (s.language === "zh" || s.language === "en") setLanguageState(s.language as Language);
    });
  }, []);

  // Apply dark class to root element
  useEffect(() => {
    if (theme === "dark") {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  }, [theme]);

  const setTheme = useCallback((t: Theme) => {
    setThemeState(t);
  }, []);

  const setLanguage = useCallback((l: Language) => {
    setLanguageState(l);
  }, []);

  const t = getTranslations(language);

  return (
    <SettingsContext.Provider value={{ theme, language, t, setTheme, setLanguage }}>
      <div className="size-full">
        <SSHLogin />
      </div>
    </SettingsContext.Provider>
  );
}