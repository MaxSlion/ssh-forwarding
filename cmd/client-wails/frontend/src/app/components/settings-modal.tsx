import { Settings, X, Save, RotateCcw } from "lucide-react";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { Label } from "./ui/label";
import { useState, useEffect } from "react";
import { LoadSettings, SaveSettings } from "../../../wailsjs/go/main/App";
import { main } from "../../../wailsjs/go/models";
import { useSettings } from "../settings-context";

interface SettingsModalProps {
    isOpen: boolean;
    onClose: () => void;
}

export function SettingsModal({ isOpen, onClose }: SettingsModalProps) {
    const { t, theme, setTheme, setLanguage } = useSettings();
    const isDark = theme === "dark";

    const [settings, setSettings] = useState<main.AppSettings>(new main.AppSettings());
    const [saving, setSaving] = useState(false);
    const [saved, setSaved] = useState(false);

    // Animation state
    const [isRendered, setIsRendered] = useState(false);
    const [isClosing, setIsClosing] = useState(false);

    useEffect(() => {
        if (isOpen) {
            setIsRendered(true);
            setIsClosing(false);
            LoadSettings().then((s) => {
                setSettings(new main.AppSettings(s));
            });
            setSaved(false);
        } else {
            if (isRendered) {
                setIsClosing(true);
                const timer = setTimeout(() => {
                    setIsRendered(false);
                    setIsClosing(false);
                }, 200); // 200ms matches CSS animation duration
                return () => clearTimeout(timer);
            }
        }
    }, [isOpen]);

    const handleSave = async () => {
        setSaving(true);
        const ok = await SaveSettings(settings);
        setSaving(false);
        if (ok) {
            // Apply settings immediately
            setTheme(settings.theme as "light" | "dark");
            setLanguage(settings.language as "zh" | "en");
            setSaved(true);
            setTimeout(() => onClose(), 600);
        }
    };

    const handleReset = () => {
        setSettings(new main.AppSettings({}));
    };

    const update = (field: string, value: any) => {
        setSettings((prev) => {
            const next = new main.AppSettings(prev);
            (next as any)[field] = value;
            return next;
        });
    };

    // Apply theme/language changes live as user selects
    const updateTheme = (value: string) => {
        update("theme", value);
        setTheme(value as "light" | "dark");
    };

    const updateLanguage = (value: string) => {
        update("language", value);
        setLanguage(value as "zh" | "en");
    };

    if (!isRendered) return null;

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
            <div
                className={`modal-backdrop absolute inset-0 bg-black/40 ${isClosing ? 'closing' : ''}`}
                onClick={onClose}
            />

            <div className={`modal-content relative rounded-lg shadow-xl w-[520px] max-h-[85vh] overflow-hidden ${isDark ? 'bg-gray-800' : 'bg-white'} ${isClosing ? 'closing' : ''
                }`}>
                {/* Header */}
                <div className={`flex items-center justify-between px-6 py-4 border-b ${isDark ? 'border-gray-700' : 'border-slate-200'
                    }`}>
                    <div className="flex items-center gap-2">
                        <Settings className={`h-5 w-5 ${isDark ? 'text-gray-300' : 'text-slate-700'}`} />
                        <h2 className={`font-semibold ${isDark ? 'text-gray-100' : 'text-slate-900'}`}>{t.settings}</h2>
                    </div>
                    <button onClick={onClose} className={`p-1 rounded ${isDark ? 'hover:bg-gray-700' : 'hover:bg-slate-100'}`}>
                        <X className={`h-4 w-4 ${isDark ? 'text-gray-400' : 'text-slate-500'}`} />
                    </button>
                </div>

                {/* Body */}
                <div className="px-6 py-5 space-y-6 overflow-auto max-h-[60vh]">

                    {/* Connection Settings */}
                    <div>
                        <h3 className={`text-sm font-semibold mb-3 uppercase tracking-wide ${isDark ? 'text-gray-300' : 'text-slate-900'}`}>
                            {t.connectionSettings}
                        </h3>
                        <div className="space-y-4">
                            <div className="space-y-1.5">
                                <Label className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>
                                    {t.agentPath}
                                </Label>
                                <Input
                                    type="text"
                                    value={settings.agentPath}
                                    onChange={(e) => update("agentPath", e.target.value)}
                                    placeholder="./server-agent"
                                    className={`h-9 font-mono text-sm ${isDark ? 'bg-gray-700 border-gray-600 text-gray-100' : 'border-slate-300'}`}
                                />
                                <p className={`text-xs ${isDark ? 'text-gray-500' : 'text-slate-500'}`}>
                                    {t.agentPathHint}
                                </p>
                            </div>

                            <div className="space-y-1.5">
                                <Label className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>
                                    {t.connectionTimeout}（{t.connectionTimeoutUnit}）
                                </Label>
                                <Input
                                    type="number"
                                    min={1}
                                    max={60}
                                    value={settings.connectionTimeout}
                                    onChange={(e) => update("connectionTimeout", parseInt(e.target.value) || 10)}
                                    className={`h-9 w-24 ${isDark ? 'bg-gray-700 border-gray-600 text-gray-100' : 'border-slate-300'}`}
                                />
                            </div>

                            <div className="space-y-1.5">
                                <Label className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>
                                    {t.localBindAddress}
                                </Label>
                                <div className="flex gap-3">
                                    <label className="flex items-center gap-2 cursor-pointer">
                                        <input
                                            type="radio"
                                            name="bindAddr"
                                            checked={settings.localBindAddress === "127.0.0.1"}
                                            onChange={() => update("localBindAddress", "127.0.0.1")}
                                            className="text-blue-600"
                                        />
                                        <span className={`text-sm font-mono ${isDark ? 'text-gray-200' : ''}`}>127.0.0.1</span>
                                        <span className={`text-xs ${isDark ? 'text-gray-500' : 'text-slate-500'}`}>（{t.localOnly}）</span>
                                    </label>
                                    <label className="flex items-center gap-2 cursor-pointer">
                                        <input
                                            type="radio"
                                            name="bindAddr"
                                            checked={settings.localBindAddress === "0.0.0.0"}
                                            onChange={() => update("localBindAddress", "0.0.0.0")}
                                            className="text-blue-600"
                                        />
                                        <span className={`text-sm font-mono ${isDark ? 'text-gray-200' : ''}`}>0.0.0.0</span>
                                        <span className={`text-xs ${isDark ? 'text-gray-500' : 'text-slate-500'}`}>（{t.lanShare}）</span>
                                    </label>
                                </div>
                            </div>

                            <div className="flex items-center justify-between">
                                <div>
                                    <Label className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>
                                        {t.autoReconnect}
                                    </Label>
                                    <p className={`text-xs ${isDark ? 'text-gray-500' : 'text-slate-500'}`}>
                                        {t.autoReconnectHint}
                                    </p>
                                </div>
                                <button
                                    onClick={() => update("autoReconnect", !settings.autoReconnect)}
                                    className={`toggle-track relative w-11 h-6 rounded-full ${settings.autoReconnect ? "bg-blue-600" : isDark ? "bg-gray-600" : "bg-slate-300"
                                        }`}
                                >
                                    <span
                                        className={`toggle-thumb absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full shadow ${settings.autoReconnect ? "translate-x-5" : "translate-x-0"
                                            }`}
                                    />
                                </button>
                            </div>
                        </div>
                    </div>

                    {/* Appearance */}
                    <div>
                        <h3 className={`text-sm font-semibold mb-3 uppercase tracking-wide ${isDark ? 'text-gray-300' : 'text-slate-900'}`}>
                            {t.appearance}
                        </h3>
                        <div className="space-y-4">
                            <div className="space-y-1.5">
                                <Label className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>{t.theme}</Label>
                                <div className="flex gap-3">
                                    <button
                                        onClick={() => updateTheme("light")}
                                        className={`flex-1 px-4 py-2.5 rounded-lg border text-sm font-medium transition-colors ${settings.theme === "light"
                                            ? "border-blue-500 bg-blue-50 text-blue-700"
                                            : isDark
                                                ? "border-gray-600 bg-gray-700 text-gray-300 hover:border-gray-500"
                                                : "border-slate-200 bg-white text-slate-600 hover:border-slate-300"
                                            }`}
                                    >
                                        ☀️ {t.themeLight}
                                    </button>
                                    <button
                                        onClick={() => updateTheme("dark")}
                                        className={`flex-1 px-4 py-2.5 rounded-lg border text-sm font-medium transition-colors ${settings.theme === "dark"
                                            ? "border-blue-500 bg-blue-900/50 text-blue-300"
                                            : isDark
                                                ? "border-gray-600 bg-gray-700 text-gray-300 hover:border-gray-500"
                                                : "border-slate-200 bg-white text-slate-600 hover:border-slate-300"
                                            }`}
                                    >
                                        🌙 {t.themeDark}
                                    </button>
                                </div>
                            </div>

                            <div className="space-y-1.5">
                                <Label className={`text-sm font-medium ${isDark ? 'text-gray-300' : 'text-slate-700'}`}>{t.language}</Label>
                                <div className="flex gap-3">
                                    <button
                                        onClick={() => updateLanguage("zh")}
                                        className={`flex-1 px-4 py-2.5 rounded-lg border text-sm font-medium transition-colors ${settings.language === "zh"
                                            ? isDark ? "border-blue-500 bg-blue-900/50 text-blue-300" : "border-blue-500 bg-blue-50 text-blue-700"
                                            : isDark
                                                ? "border-gray-600 bg-gray-700 text-gray-300 hover:border-gray-500"
                                                : "border-slate-200 bg-white text-slate-600 hover:border-slate-300"
                                            }`}
                                    >
                                        中文
                                    </button>
                                    <button
                                        onClick={() => updateLanguage("en")}
                                        className={`flex-1 px-4 py-2.5 rounded-lg border text-sm font-medium transition-colors ${settings.language === "en"
                                            ? isDark ? "border-blue-500 bg-blue-900/50 text-blue-300" : "border-blue-500 bg-blue-50 text-blue-700"
                                            : isDark
                                                ? "border-gray-600 bg-gray-700 text-gray-300 hover:border-gray-500"
                                                : "border-slate-200 bg-white text-slate-600 hover:border-slate-300"
                                            }`}
                                    >
                                        English
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                {/* Footer */}
                <div className={`px-6 py-4 border-t flex items-center justify-between ${isDark ? 'border-gray-700 bg-gray-750' : 'border-slate-200 bg-slate-50'
                    }`}>
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={handleReset}
                        className={`gap-1.5 ${isDark ? 'border-gray-600 text-gray-300 hover:bg-gray-700' : ''}`}
                    >
                        <RotateCcw className="h-3.5 w-3.5" />
                        {t.resetDefaults}
                    </Button>
                    <div className="flex gap-2">
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={onClose}
                            className={isDark ? 'border-gray-600 text-gray-300 hover:bg-gray-700' : ''}
                        >
                            {t.cancel}
                        </Button>
                        <Button
                            size="sm"
                            onClick={handleSave}
                            disabled={saving}
                            className="gap-1.5 bg-blue-600 hover:bg-blue-700 text-white"
                        >
                            <Save className="h-3.5 w-3.5" />
                            {saved ? t.saved : saving ? t.saving : t.save}
                        </Button>
                    </div>
                </div>
            </div>
        </div>
    );
}
