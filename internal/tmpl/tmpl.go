package tmpl

import (
    "html/template"
)

func Load() *template.Template {
    funcMap := template.FuncMap{
        "map": func(pairs ...any) map[string]any {
            m := make(map[string]any)
            for i := 0; i+1 < len(pairs); i += 2 {
                key, _ := pairs[i].(string)
                m[key] = pairs[i+1]
            }
            return m
        },
    }
    return template.Must(
        template.New("").Funcs(funcMap).ParseGlob("templates/*.html"),
    )
}