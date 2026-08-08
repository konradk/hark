import QtQuick
import Quickshell
import Quickshell.Hyprland
import Quickshell.Io

QtObject {
    id: root

    readonly property string home: Quickshell.env("HOME")
    readonly property string currentThemePath: home + "/.local/state/omarchy/current/theme"
    readonly property bool omarchyAvailable: omarchyPalette.background !== undefined && omarchyPalette.foreground !== undefined
    readonly property var palette: omarchyAvailable ? omarchyPalette : systemPalette
    readonly property bool available: palette.background !== undefined && palette.foreground !== undefined
    readonly property int baseFontSize: Math.max(1, Number(shellValues["font.base-size"] ?? 12))
    property int cornerRadius: 0
    property string fontFamily: "monospace"
    property var omarchyPalette: ({})
    property bool prefersDark: true
    property string systemAccent: ""
    property int systemBorderWidth: -1
    readonly property var systemPalette: ({
        "background": prefersDark ? "#15171e" : "#f4f5f7",
        "foreground": prefersDark ? "#f2f4f8" : "#1b1f28",
        "accent": systemAccent.length > 0 ? systemAccent : (prefersDark ? "#a7c7ff" : "#3b6ea5"),
        "red": prefersDark ? "#e07878" : "#c0392b",
        "bright_red": prefersDark ? "#ffb4b4" : "#8c2f22"
    })
    property var themeShellValues: ({})
    property var userShellValues: ({})
    property var shellValues: ({})
    property var theme: ({})

    function parseColors(raw) {
        const parsed = {};
        const lines = String(raw ?? "").split("\n");
        for (const line of lines) {
            const match = line.match(/^\s*([A-Za-z0-9_-]+)\s*=\s*["']?([^"'#\s][^"'#]*|#[0-9A-Fa-f]{6})/);
            if (!match)
                continue;

            parsed[match[1]] = match[2].trim();
        }
        if (!parsed.background && parsed.color0)
            parsed.background = parsed.color0;

        if (!parsed.foreground && parsed.color7)
            parsed.foreground = parsed.color7;

        if (!parsed.accent)
            parsed.accent = parsed.color4 ?? parsed.foreground;

        if (!parsed.muted)
            parsed.muted = parsed.color8 ?? parsed.foreground;

        if (!parsed.red)
            parsed.red = parsed.color1 ?? parsed.foreground;

        if (!parsed.bright_red)
            parsed.bright_red = parsed.red;

        if (!parsed.darker_background)
            parsed.darker_background = parsed.background;

        if (!parsed.lighter_background)
            parsed.lighter_background = parsed.background;

        if (!parsed.selection)
            parsed.selection = parsed.accent;

        return parsed;
    }

    function parseShell(raw) {
        const parsed = {};
        const lines = String(raw ?? "").split("\n");
        let section = "";
        for (const sourceLine of lines) {
            const line = sourceLine.trim();
            if (line.length === 0 || line.startsWith("#"))
                continue;

            const sectionMatch = line.match(/^\[([A-Za-z0-9_-]+)\]\s*(?:#.*)?$/);
            if (sectionMatch) {
                section = sectionMatch[1];
                continue;
            }
            if (section.length === 0)
                continue;

            const quoted = line.match(/^([A-Za-z0-9_-]+)\s*=\s*["']([^"']+)["']\s*(?:#.*)?$/);
            const bare = line.match(/^([A-Za-z0-9_-]+)\s*=\s*([^#\s]+)\s*(?:#.*)?$/);
            const match = quoted ?? bare;
            if (match)
                parsed[section + "." + match[1]] = match[2];

        }
        return parsed;
    }

    function mergeShell() {
        const merged = {};
        for (const key in themeShellValues)
            merged[key] = themeShellValues[key];

        for (const key in userShellValues)
            merged[key] = userShellValues[key];

        shellValues = merged;
        rebuild();
    }

    function firstColorToken(value) {
        const tokens = String(value ?? "").trim().split(/\s+/);
        for (const token of tokens) {
            if (!token.match(/^-?\d+(?:\.\d+)?deg$/))
                return token;

        }
        return String(value ?? "");
    }

    function flatColor(value, fallback, depth) {
        const token = firstColorToken(value);
        const role = token.trim().toLowerCase();
        const level = depth ?? 0;
        if (level < 6 && shellValues[role] && shellValues[role] !== token)
            return flatColor(shellValues[role], fallback, level + 1);

        if (palette[role])
            return palette[role];

        const rgba = token.match(/^rgba\(([0-9A-Fa-f]{8})\)$/);
        if (rgba)
            return "#" + rgba[1].slice(0, 6);

        if (token.match(/^#[0-9A-Fa-f]{6}$/))
            return token;

        return fallback;
    }

    function numberValue(value, fallback) {
        const result = Number(value);
        return isFinite(result) ? result : fallback;
    }

    function fontToken(key, multiplier) {
        const configured = Number(shellValues["font." + key]);
        if (isFinite(configured) && configured > 0)
            return Math.round(configured);

        return Math.max(1, Math.round(baseFontSize * multiplier));
    }

    function alpha(color, opacity) {
        const match = String(color ?? "").match(/^#([0-9A-Fa-f]{6})$/);
        if (!match)
            return color;

        const hex = match[1];
        return Qt.rgba(parseInt(hex.slice(0, 2), 16) / 255, parseInt(hex.slice(2, 4), 16) / 255, parseInt(hex.slice(4, 6), 16) / 255, Math.max(0, Math.min(1, opacity)));
    }

    function isDarkColor(value) {
        const parsed = Qt.color(value);
        return 0.2126 * parsed.r + 0.7152 * parsed.g + 0.0722 * parsed.b < 0.5;
    }

    function pick(key, fallback) {
        const value = shellValues[key];
        return typeof value === "string" && value.length > 0 ? value : fallback;
    }

    function surface(section, key, fallback, fallbackAlpha) {
        const colorKey = section + "." + key;
        const color = flatColor(pick(colorKey, fallback), fallback);
        const opacity = numberValue(pick(colorKey + "-alpha", String(fallbackAlpha)), fallbackAlpha);
        return alpha(color, opacity);
    }

    function rebuild() {
        if (!available)
            return;

        const background = palette.background;
        const foreground = palette.foreground;
        const accent = palette.accent;
        const error = palette.red;
        const errorText = palette.bright_red;
        const normalAlpha = numberValue(pick("controls.normal-fill-alpha", "0.04"), 0.04);
        const hoverAlpha = numberValue(pick("controls.hover-cursor-fill-alpha", "0.08"), 0.08);
        const pressedAlpha = numberValue(pick("controls.pressed-fill-alpha", "0.22"), 0.22);
        const selectedAlpha = numberValue(pick("controls.selected-fill-alpha", "0.18"), 0.18);
        const controlForeground = flatColor(pick("controls.normal-color", foreground), foreground);
        const selectedForeground = flatColor(pick("controls.selected-color", controlForeground), controlForeground);
        const onDark = isDarkColor(background);
        const dimForeground = onDark ? Qt.darker(foreground, 1.4) : Qt.lighter(foreground, 1.8);
        const disabledForeground = onDark ? Qt.darker(foreground, 2) : Qt.lighter(foreground, 2.6);
        const menuFontOverride = Quickshell.env("OMARCHY_MENU_FONT");

        theme = {
            "background": "transparent",
            "panel": surface("popups", "background", background, 1),
            "panel_border": surface("popups", "border", accent, 1),
            "panel_border_width": Math.max(0, numberValue(pick("popups.border-width", String(systemBorderWidth >= 0 ? systemBorderWidth : 2)), 2)),
            "corner_radius": cornerRadius,
            "font_family": menuFontOverride && menuFontOverride.length > 0 ? menuFontOverride : fontFamily,
            "font_caption": fontToken("caption", 0.833),
            "font_body_small": fontToken("body-small", 0.917),
            "font_body": fontToken("body", 1),
            "font_subtitle": fontToken("subtitle", 1.083),
            "font_title": fontToken("title", 1.167),
            "font_heading": fontToken("heading", 1.333),
            "font_display": fontToken("display", 2),
            "font_display_large": fontToken("display-large", 2.333),
            "font_icon_small": fontToken("icon-small", 0.917),
            "font_icon": fontToken("icon", 1.167),
            "font_icon_large": fontToken("icon-large", 1.5),
            "surface": alpha(controlForeground, normalAlpha),
            "surface_elevated": surface("popups", "background", background, 1),
            "surface_hover": alpha(controlForeground, hoverAlpha),
            "surface_active": alpha(selectedForeground, selectedAlpha),
            "input": alpha(controlForeground, normalAlpha),
            "button": alpha(controlForeground, normalAlpha),
            "button_disabled": alpha(controlForeground, normalAlpha / 2),
            "button_hover": alpha(controlForeground, hoverAlpha),
            "button_down": alpha(controlForeground, pressedAlpha),
            "primary": selectedForeground,
            "primary_hover": controlForeground,
            "primary_down": controlForeground,
            "primary_text": background,
            "text": flatColor(pick("popups.text", foreground), foreground),
            "text_strong": foreground,
            "text_muted": dimForeground,
            "text_disabled": disabledForeground,
            "assistant": foreground,
            "error": error,
            "error_text": errorText,
            "error_surface": alpha(error, 0.12),
            "error_border": alpha(error, 0.55),
            "selection_text": background
        };
    }

    function refreshSystemStyle() {
        if (!roundingProcess.running)
            roundingProcess.running = true;

        if (!borderColorProcess.running)
            borderColorProcess.running = true;

        if (!borderWidthProcess.running)
            borderWidthProcess.running = true;

    }

    function refreshColorScheme() {
        if (!colorSchemeProcess.running)
            colorSchemeProcess.running = true;

    }

    function applyColorScheme(raw) {
        // Portal values: 1 dark, 2 light, 0 unspecified.
        const match = String(raw ?? "").match(/uint32\s+(\d+)/);
        if (!match)
            return ;

        const next = Number(match[1]) !== 2;
        if (next === prefersDark)
            return ;

        prefersDark = next;
        rebuild();
    }

    function applyBorderColor(raw) {
        // Hyprland gradients start with an AARRGGBB stop.
        try {
            const resolved = JSON.parse(String(raw ?? "{}"));
            const stop = String(resolved.gradient ?? "").trim().match(/^([0-9A-Fa-f]{8})/);
            if (!stop)
                return ;

            const next = "#" + stop[1].slice(2);
            if (next === systemAccent)
                return ;

            systemAccent = next;
            rebuild();
        } catch (error) {
        }
    }

    function applyBorderWidth(raw) {
        try {
            const resolved = JSON.parse(String(raw ?? "{}"));
            const value = Number(resolved.int);
            if (!isFinite(value) || value < 0 || value === systemBorderWidth)
                return ;

            systemBorderWidth = value;
            rebuild();
        } catch (error) {
        }
    }

    function resolveFontFamily() {
        if (!fontFamilyProcess.running)
            fontFamilyProcess.running = true;

    }

    function applyRounding(raw) {
        try {
            const resolved = JSON.parse(String(raw ?? "{}"));
            const value = Number(resolved.int);
            if (isFinite(value) && value >= 0) {
                cornerRadius = value;
                rebuild();
            }
        } catch (error) {
        }
    }

    function reloadThemeFiles() {
        colorsFile.reload();
        themeShellFile.reload();
        styleRefreshTimer.restart();
    }

    property Timer reloadTimer: Timer {
        interval: 50
        repeat: false
        onTriggered: root.reloadThemeFiles()
    }

    property Timer styleRefreshTimer: Timer {
        interval: 200
        repeat: false
        onTriggered: root.refreshSystemStyle()
    }

    property Process roundingProcess: Process {
        command: ["hyprctl", "-j", "getoption", "decoration:rounding"]

        stdout: StdioCollector {
            waitForEnd: true
            onStreamFinished: root.applyRounding(text)
        }
    }

    property Process borderColorProcess: Process {
        command: ["hyprctl", "-j", "getoption", "general:col.active_border"]

        stdout: StdioCollector {
            waitForEnd: true
            onStreamFinished: root.applyBorderColor(text)
        }
    }

    property Process borderWidthProcess: Process {
        command: ["hyprctl", "-j", "getoption", "general:border_size"]

        stdout: StdioCollector {
            waitForEnd: true
            onStreamFinished: root.applyBorderWidth(text)
        }
    }

    property Process colorSchemeProcess: Process {
        command: ["gdbus", "call", "--session", "--dest", "org.freedesktop.portal.Desktop", "--object-path", "/org/freedesktop/portal/desktop", "--method", "org.freedesktop.portal.Settings.ReadOne", "org.freedesktop.appearance", "color-scheme"]

        stdout: StdioCollector {
            waitForEnd: true
            onStreamFinished: root.applyColorScheme(text)
        }
    }

    property Process fontFamilyProcess: Process {
        command: ["fc-match", "-f", "%{family[0]}", "monospace"]

        stdout: StdioCollector {
            waitForEnd: true
            onStreamFinished: {
                const resolved = String(text ?? "").trim();
                if (resolved.length > 0) {
                    root.fontFamily = resolved;
                    root.rebuild();
                }
            }
        }
    }

    property Connections hyprlandEvents: Connections {
        target: Hyprland

        function onRawEvent(event) {
            if (event && String(event.name ?? "") === "configreloaded")
                root.styleRefreshTimer.restart();

        }
    }

    property FileView lookAndFeelFile: FileView {
        path: root.home + "/.config/hypr/looknfeel.lua"
        watchChanges: true
        printErrors: false
        onLoaded: root.styleRefreshTimer.restart()
        onFileChanged: root.styleRefreshTimer.restart()
    }

    property FileView fontconfigFile: FileView {
        path: root.home + "/.config/fontconfig/fonts.conf"
        watchChanges: true
        printErrors: false
        onLoaded: root.resolveFontFamily()
        onFileChanged: root.resolveFontFamily()
        onLoadFailed: root.resolveFontFamily()
    }

    property FileView colorsFile: FileView {
        path: root.currentThemePath + "/colors.toml"
        watchChanges: true
        printErrors: false
        onLoaded: {
            root.omarchyPalette = root.parseColors(text());
            root.rebuild();
        }
        onLoadFailed: {
            root.omarchyPalette = {};
            root.rebuild();
        }
        onFileChanged: reload()
    }

    property FileView themeShellFile: FileView {
        path: root.currentThemePath + "/shell.toml"
        watchChanges: true
        printErrors: false
        onLoaded: {
            root.themeShellValues = root.parseShell(text());
            root.mergeShell();
        }
        onLoadFailed: {
            root.themeShellValues = {};
            root.mergeShell();
        }
        onFileChanged: reload()
    }

    property FileView userShellFile: FileView {
        path: root.home + "/.config/omarchy/shell.toml"
        watchChanges: true
        printErrors: false
        onLoaded: {
            root.userShellValues = root.parseShell(text());
            root.mergeShell();
        }
        onLoadFailed: {
            root.userShellValues = {};
            root.mergeShell();
        }
        onFileChanged: reload()
    }

    property FileView themeNameFile: FileView {
        path: root.home + "/.local/state/omarchy/current/theme.name"
        watchChanges: true
        printErrors: false
        onLoaded: root.reloadTimer.restart()
        onFileChanged: reload()
    }

    Component.onCompleted: {
        refreshSystemStyle();
        refreshColorScheme();
        resolveFontFamily();
    }
}
