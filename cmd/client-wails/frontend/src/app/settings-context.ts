import { createContext, useContext } from "react";
import { type Language, type Translations, getTranslations } from "./i18n";

export type Theme = "light" | "dark";

export interface AppSettingsContext {
    theme: Theme;
    language: Language;
    t: Translations;
    setTheme: (theme: Theme) => void;
    setLanguage: (lang: Language) => void;
}

const defaultCtx: AppSettingsContext = {
    theme: "light",
    language: "zh",
    t: getTranslations("zh"),
    setTheme: () => { },
    setLanguage: () => { },
};

export const SettingsContext = createContext<AppSettingsContext>(defaultCtx);

export function useSettings() {
    return useContext(SettingsContext);
}
