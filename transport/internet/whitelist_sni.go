package internet

import (
	"strings"
	"sync"
)

// WhitelistSNI содержит список доменов из whitelist РКН
// Эти домены не блокируются и могут использоваться для маскировки
var WhitelistSNI = struct {
	sync.RWMutex
	domains []string
}{
	domains: []string{
		// Российские сервисы (не блокируются)
		"mail.ru",
		"yandex.ru",
		"vk.com",
		"ok.ru",
		"sberbank.ru",
		"gosuslugi.ru",
		"mos.ru",
		"nalog.ru",
		"tinkoff.ru",
		"avito.ru",
		"wildberries.ru",
		"ozon.ru",
		"2gis.ru",
		"kinopoisk.ru",
		"ivi.ru",
		"megafon.ru",
		"mts.ru",
		"beeline.ru",
		"tele2.ru",
		"rt.ru",
		
		// Образовательные ресурсы
		"msu.ru",
		"spbu.ru",
		"hse.ru",
		"mipt.ru",
		"mephi.ru",
		"edu.ru",
		
		// Международные сервисы с российской инфраструктурой
		"microsoft.com",
		"apple.com",
		"google.com",
		"cloudflare.com",
		"akamai.com",
		"amazon.com",
		"netflix.com",
		"spotify.com",
		
		// CDN и облачные сервисы
		"cloudflare.net",
		"akamaiedge.net",
		"amazonaws.com",
		"azureedge.net",
		"fastly.net",
		"stackpath.com",
		
		// Популярные международные сайты (часто в whitelist)
		"wikipedia.org",
		"github.com",
		"stackoverflow.com",
		"reddit.com",
		"medium.com",
		"wordpress.com",
		"adobe.com",
		"oracle.com",
		"ibm.com",
		"intel.com",
		"nvidia.com",
		"amd.com",
		
		// Игровые платформы
		"steampowered.com",
		"epicgames.com",
		"ea.com",
		"ubisoft.com",
		"blizzard.com",
		"riotgames.com",
		"minecraft.net",
		
		// Мессенджеры и соцсети (частично)
		"whatsapp.com",
		"telegram.org",
		"discord.com",
		"slack.com",
		"zoom.us",
		"skype.com",
	},
}

// GetRandomWhitelistSNI возвращает случайный домен из whitelist
func GetRandomWhitelistSNI() string {
	WhitelistSNI.RLock()
	defer WhitelistSNI.RUnlock()
	
	if len(WhitelistSNI.domains) == 0 {
		return "yandex.ru" // По умолчанию
	}
	
	// Выбираем случайный домен
	idx := randInt(0, len(WhitelistSNI.domains))
	return WhitelistSNI.domains[idx]
}

// GetWhitelistSNIForDomain возвращает подходящий whitelist SNI для домена
func GetWhitelistSNIForDomain(domain string) string {
	// Пытаемся подобрать похожий домен из whitelist
	domain = strings.ToLower(domain)
	
	WhitelistSNI.RLock()
	defer WhitelistSNI.RUnlock()
	
	// Проверяем, есть ли сам домен в whitelist
	for _, wl := range WhitelistSNI.domains {
		if strings.Contains(domain, wl) || strings.Contains(wl, domain) {
			return wl
		}
	}
	
	// Подбираем по категории
	if strings.Contains(domain, "cdn") || strings.Contains(domain, "static") {
		return "cloudflare.net"
	}
	
	if strings.Contains(domain, "api") {
		return "googleapis.com"
	}
	
	if strings.Contains(domain, "video") || strings.Contains(domain, "stream") {
		return "youtube.com"
	}
	
	if strings.Contains(domain, "chat") || strings.Contains(domain, "messenger") {
		return "telegram.org"
	}
	
	// По умолчанию возвращаем популярный российский сервис
	popularRussian := []string{
		"yandex.ru",
		"mail.ru",
		"vk.com",
		"sberbank.ru",
		"wildberries.ru",
	}
	
	return popularRussian[randInt(0, len(popularRussian))]
}

// IsInWhitelist проверяет, находится ли домен в whitelist
func IsInWhitelist(domain string) bool {
	domain = strings.ToLower(domain)
	
	WhitelistSNI.RLock()
	defer WhitelistSNI.RUnlock()
	
	for _, wl := range WhitelistSNI.domains {
		if domain == wl || strings.HasSuffix(domain, "."+wl) {
			return true
		}
	}
	
	return false
}

// AddToWhitelist добавляет домен в whitelist
func AddToWhitelist(domain string) {
	WhitelistSNI.Lock()
	defer WhitelistSNI.Unlock()
	
	// Проверяем, что домена еще нет в списке
	domain = strings.ToLower(domain)
	for _, existing := range WhitelistSNI.domains {
		if existing == domain {
			return
		}
	}
	
	WhitelistSNI.domains = append(WhitelistSNI.domains, domain)
}

// RemoveFromWhitelist удаляет домен из whitelist
func RemoveFromWhitelist(domain string) {
	WhitelistSNI.Lock()
	defer WhitelistSNI.Unlock()
	
	domain = strings.ToLower(domain)
	newDomains := make([]string, 0, len(WhitelistSNI.domains))
	
	for _, existing := range WhitelistSNI.domains {
		if existing != domain {
			newDomains = append(newDomains, existing)
		}
	}
	
	WhitelistSNI.domains = newDomains
}

// GetWhitelistDomains возвращает все домены из whitelist
func GetWhitelistDomains() []string {
	WhitelistSNI.RLock()
	defer WhitelistSNI.RUnlock()
	
	result := make([]string, len(WhitelistSNI.domains))
	copy(result, WhitelistSNI.domains)
	return result
}

// SNIMasker маскирует SNI используя whitelist
type SNIMasker struct {
	realSNI      string
	maskedSNI    string
	useWhitelist bool
}

// NewSNIMasker создает новый маскировщик SNI
func NewSNIMasker(realSNI string, useWhitelist bool) *SNIMasker {
	masker := &SNIMasker{
		realSNI:      realSNI,
		useWhitelist: useWhitelist,
	}
	
	if useWhitelist {
		// Используем домен из whitelist для маскировки
		masker.maskedSNI = GetWhitelistSNIForDomain(realSNI)
	} else {
		masker.maskedSNI = realSNI
	}
	
	return masker
}

// GetMaskedSNI возвращает замаскированный SNI
func (sm *SNIMasker) GetMaskedSNI() string {
	return sm.maskedSNI
}

// GetRealSNI возвращает реальный SNI
func (sm *SNIMasker) GetRealSNI() string {
	return sm.realSNI
}

// ShouldMaskSNI определяет, нужно ли маскировать SNI
func ShouldMaskSNI(domain string) bool {
	// Не маскируем домены из whitelist
	if IsInWhitelist(domain) {
		return false
	}
	
	// Маскируем все остальные домены
	return true
}