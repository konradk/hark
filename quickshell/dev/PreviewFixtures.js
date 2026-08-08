.pragma library

function longThread() {
    return {
        "prompt": "tell me about wroclaw",
        "answer": "Wroclaw is one of Poland's most attractive and historic cities, located in the southwest on the Oder River. It is the capital of the Lower Silesian region and is known for colorful architecture, lively student culture, bridges, islands, and a layered Central European history.\n\n**What Wroclaw is known for**\n\n- **Beautiful Old Town:** The Market Square, or Rynek, is one of the largest and prettiest in Poland, with pastel townhouses and a striking Gothic-style Old Town Hall.\n- **Oder River, islands, and bridges:** Wroclaw is sometimes nicknamed the Venice of Poland because of its many waterways and bridges.\n- **Dwarfs:** Hundreds of small bronze dwarf statues are scattered around the city. They began as a symbol of anti-communist resistance and are now a beloved tourist attraction.\n- **Cathedral Island:** Ostrow Tumski is the oldest part of Wroclaw, with historic churches, cobbled streets, and gas lamps that are still lit by hand in the evening.\n- **Centennial Hall:** A UNESCO World Heritage Site and an important work of early modernist architecture.\n- **University city atmosphere:** Wroclaw has a large student population, which gives it a youthful, energetic feel with many cafes, bars, cultural events, and nightlife."
    };
}

function code() {
    return {
        "prompt": "show me shell and go examples",
        "answer": "Here is a compact shell example:\n\n```bash\nfind . -maxdepth 2 -type f | sort\n```\n\nAnd a Go helper:\n\n```go\nfunc greet(name string) string {\n    return \"hello, \" + name\n}\n```\n\nBoth code blocks should have clear labels, selectable text, and visible copy actions."
    };
}

function markdown() {
    return {
        "prompt": "show me every markdown feature",
        "answer": "Markdown showcase with **bold**, *italic*, ~~strikethrough~~, `inline code`, and a [link to the docs](https://example.com/docs).\n\n# Heading one\n\nA paragraph under the largest heading.\n\n## Heading two\n\n- Bullet item with **bold** text\n- Bullet item with `inline code`\n  - Nested bullet item\n  - Another nested item\n- Third top-level item\n\n### Heading three\n\n1. First ordered step\n2. Second ordered step\n3. Third ordered step\n\n> A blockquote spanning a couple of lines, useful for quoting documentation or previous answers.\n\n---\n\n| Feature | Status | Notes |\n| --- | --- | --- |\n| Tables | works | GFM pipe tables |\n| Lists | works | nested too |\n| Code | works | fenced blocks |\n\n```python\ndef greet(name: str) -> str:\n    return f\"hello, {name}\"\n```\n\nClosing paragraph after the code block to verify spacing."
    };
}

function history() {
    return {
        "prompt": "what did I ask recently?",
        "answer": "History should be compact and secondary while the active answer remains readable.",
        "entries": [
            {
                "entryId": 101,
                "prompt": "summarize this stack trace",
                "response": "This stack trace points to a nil value in the request path."
            },
            {
                "entryId": 102,
                "prompt": "write a short release note",
                "response": "Improved Wayland paste-back and screenshot handling."
            },
            {
                "entryId": 103,
                "prompt": "compare quickshell and ags",
                "response": "Quickshell is a better fit for a native Hyprland palette."
            }
        ]
    };
}

function demoHistory() {
    return {
        "entries": [
            {
                "entryId": 201,
                "prompt": "Plan a quiet weekend in Wroclaw",
                "response": "A two-day route through Nadodrze, Ostrow Tumski, and the riverside parks."
            },
            {
                "entryId": 202,
                "prompt": "Explain this Go race condition",
                "response": "Two goroutines mutate the same map after the read lock is released."
            },
            {
                "entryId": 203,
                "prompt": "Summarize the attached architecture diagram",
                "response": "The UI talks to one local daemon through a bounded Unix-socket protocol."
            },
            {
                "entryId": 204,
                "prompt": "Draft release notes for v0.3.0",
                "response": "Safer clipboard handling, model-aware reasoning, and improved plugin lifecycle."
            },
            {
                "entryId": 205,
                "prompt": "Compare Tokyo Night and Nord",
                "response": "Tokyo Night is more saturated; Nord is cooler and more restrained."
            }
        ]
    };
}

function demoConversation() {
    return {
        "prompt": "How would you harden this local IPC service?",
        "answer": "Start with the trust boundary around the Unix socket:\n\n- create the socket directory with **0700** permissions\n- reject unknown JSON fields and enforce request-size limits\n- keep prompts, clipboard text, and secrets on **stdin**\n- cancel provider requests as soon as the client disconnects\n\nFor file attachments, accept only regular files from a dedicated cache directory and open them with `O_NOFOLLOW`. This keeps the IPC endpoint from becoming an arbitrary file reader."
    };
}

function demoAttachment() {
    return {
        "prompt": "What is causing the highlighted error in this terminal?",
        "answer": "The process cannot create its socket because the parent directory is owned by another user. Recreate the runtime directory with the current user as owner and permissions set to `0700`, then restart the service. Avoid solving this with `sudo` because that would leave root-owned runtime files behind."
    };
}

function theme(name) {
    const themes = {
        "tokyo-night": {
            "backdrop": "#0e0e14",
            "colors": palette("#1a1b26", "#0e0e14", "#24283b", "#7aa2f7", "#c0caf5", "#a9b1d6", "#565f89", "#f7768e", "#ff7a93", "#292e42")
        },
        "catppuccin-latte": {
            "backdrop": "#d7d8dc",
            "colors": palette("#eff1f5", "#e3e4e8", "#dce0e8", "#1e66f5", "#4c4f69", "#5c5f77", "#9ca0b0", "#d20f39", "#d20f39", "#ccd0da")
        },
        "solitude": {
            "backdrop": "#080a0b",
            "colors": palette("#101315", "#0c0e10", "#202528", "#798186", "#cacccc", "#a5aeb4", "#4b4e55", "#de6145", "#de6145", "#343d41")
        },
        "nord": {
            "backdrop": "#191c23",
            "colors": palette("#2e3440", "#222730", "#3b4252", "#81a1c1", "#d8dee9", "#adb5c4", "#667080", "#bf616a", "#bf616a", "#434c5e")
        }
    };
    return themes[name] ?? themes["tokyo-night"];
}

function palette(panel, dark, raised, accent, strong, text, muted, error, errorText, selection) {
    return {
        "background": "transparent",
        "panel": panel,
        "panel_border": accent,
        "panel_border_width": 2,
        "corner_radius": 14,
        "surface": dark,
        "surface_elevated": raised,
        "surface_hover": raised,
        "surface_active": selection,
        "input": dark,
        "button": dark,
        "button_disabled": panel,
        "button_hover": raised,
        "button_down": selection,
        "primary": accent,
        "primary_hover": accent,
        "primary_down": accent,
        "primary_text": panel,
        "text": text,
        "text_strong": strong,
        "text_muted": muted,
        "text_disabled": muted,
        "assistant": strong,
        "error": error,
        "error_text": errorText,
        "error_surface": dark,
        "error_border": error,
        "selection_text": panel
    };
}
