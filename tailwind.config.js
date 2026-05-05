module.exports = {

    darkMode: "class",

    content: ["./src/ui/**/*.templ", "./src/ui/**/*.ts"],

    theme: {
        extend: {
            colors: {
                'c': {
                    "black"  : "#1F1F1F",
                    "red"    : "#F44747",
                    "green"  : "#6A9955",
                    "yellow" : "#D7BA7D",
                    "blue"   : "#569CD6",
                    "magenta": "#C586C0",
                    "cyan"   : "#4EC9B0",
                    "white"  : "#D4D4D4",
                    'l': {
                        "black"  : "#808080",
                        "red"    : "#D16969",
                        "green"  : "#B5CEA8",
                        "yellow" : "#DCDCAA",
                        "blue"   : "#9CDCFE",
                        "magenta": "#C586C0",
                        "cyan"   : "#4FC1FF",
                        "white"  : "#FFD602"
                    }
                },
            }
        }
    },

    corePlugins: {
        preflight: true,
    },
    experimental: {
        optimizeUniversalDefaults: true
    }
};
