// SPDX-License-Identifier: AGPL-3.0-or-later

package news

import (
	"os"
	"strings"
	"testing"
	"time"
)

const preview = `<html><body><section>
<div class="tgme_widget_message_wrap js-widget_message_wrap"><div class="tgme_widget_message js-widget_message" data-post="chan/101">
  <div class="tgme_widget_message_bubble">
    <div class="tgme_widget_message_reply"><div class="tgme_widget_message_text">quoted reply text</div></div>
    <div class="tgme_widget_message_text js-message_text" dir="auto"><b>Заголовок допису</b><br/><br/><br/>Перший   рядок тексту.<br/>Другий <a href="https://x">рядок</a> <i class="emoji"><b>⚡</b></i></div>
    <div class="tgme_widget_message_footer"><a class="tgme_widget_message_date" href="https://t.me/chan/101"><time datetime="2026-10-12T10:13:20+00:00" class="time">13:13</time></a></div>
  </div>
</div></div>
<div class="tgme_widget_message_wrap"><div class="tgme_widget_message" data-post="chan/102">
  <div class="tgme_widget_message_bubble"><div class="tgme_widget_message_photo_wrap"></div>
  <a class="tgme_widget_message_date"><time datetime="2026-10-12T11:00:00+00:00"></time></a></div>
</div></div>
</section></body></html>`

func TestParsePreview(t *testing.T) {
	posts, err := parsePreview(strings.NewReader(preview))
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1 (photo-only post skipped): %+v", len(posts), posts)
	}
	p := posts[0]
	want := "Заголовок допису\n\nПерший рядок тексту.\nДругий рядок ⚡"
	if p.ID != "chan/101" || p.URL != "https://t.me/chan/101" || p.Text != want {
		t.Errorf("post = %+v\nwant text %q", p, want)
	}
	if !p.PublishedAt.Equal(time.Date(2026, 10, 12, 10, 13, 20, 0, time.UTC)) {
		t.Errorf("time = %v", p.PublishedAt)
	}
}

func TestSplitTitle(t *testing.T) {
	title, rest := splitTitle("Коротко\nдалі текст")
	if title != "Коротко" || rest != "далі текст" {
		t.Errorf("%q / %q", title, rest)
	}
	long := strings.Repeat("слово ", 40)
	title, rest = splitTitle(long)
	if !strings.HasSuffix(title, "…") || len([]rune(title)) > titleRunes+1 || rest == "" {
		t.Errorf("long first line: title %q", title)
	}
}

// TestRealPreview parses a saved t.me/s page when NEWS_PREVIEW_FILE is set (manual check).
func TestRealPreview(t *testing.T) {
	path := os.Getenv("NEWS_PREVIEW_FILE")
	if path == "" {
		t.Skip("NEWS_PREVIEW_FILE not set")
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	posts, err := parsePreview(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range posts {
		title, _ := splitTitle(p.Text)
		t.Logf("%s %s | %s", p.PublishedAt.Format("15:04"), p.ID, title)
	}
	if len(posts) == 0 {
		t.Fatal("no posts")
	}
}

func TestIsThreatTracking(t *testing.T) {
	cases := map[string]bool{
		"🔺 Чернігівщина: реактивний БпЛА повз Славутич курсом на Київщину.":                             true,
		"🔻Нові групи реактивних БпЛА з Брянської області на північ Чернігівщини, рухаються на південь.": true,
		"🟢 Ніжинський район — відбій повітряної тривоги":                                                true,
		"🟢 Ніжинський район — відбій повітряної тривоги\n\n⚠️ Зверніть увагу, повітряна тривога досі триває у:\n- Корюківський район\n- Новгород-Сіверський район\n- Чернігівський район\n" + strings.Repeat("Підписатися на канал. ", 20): true,
		"🔻Нові групи реактивних БпЛА з Брянської області на північ Чернігівщини.\n\nЗа інформацією Повітряних сил ЗСУ.":                                                                                                                    true,
		"💢У Козелецькій громаді через російську атаку БпЛА пошкоджений нежилий будинок та авто.":                                                                                                                                           false,
		"🍁 11 жовтня у Чернігові зафіксували температурний рекорд на рівні +22,9°С.":                                                                                                                                                       false,
	}
	for text, want := range cases {
		if got := isThreatTracking(text); got != want {
			t.Errorf("%q: %v, want %v", text, got, want)
		}
	}
}

func TestDefaultSourcesUnique(t *testing.T) {
	seen := map[string]int{}
	for region, chans := range DefaultSources() {
		for _, c := range chans {
			u := strings.ToLower(c.Username)
			if r, ok := seen[u]; ok {
				t.Errorf("channel %s is listed for regions %d and %d", c.Username, r, region)
			}
			seen[u] = region
		}
	}
}
