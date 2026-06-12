# Self-hosting

mqvi'yi kendi altyapında çalıştırabilirsin. **İki seviye** var — ne kadar bağımsız olmak istediğine göre seç.

## İki yol

**1. Sadece ses sunucusu** *(en kolayı)*
mqvi.net hesabını kullanmaya devam et — arkadaşlar, DM'ler ve sunucular her zamanki gibi çalışır — ama **sesini ve görüntünü** kendi sunucun üzerinden yönlendir. Aramaların hiçbir zaman bizim altyapımıza dokunmaz. Kurulum tek komut.

**2. Tam sunucu** *(tamamen bağımsız)*
**Tüm platformu** kendin çalıştır — hesaplar, mesajlar, dosyalar ve ses. mqvi.net'ten tamamen ayrı; her şeyi sen kontrol edersin. Kurulum tek komut.

| | Sadece ses | Tam sunucu |
| --- | --- | --- |
| Hesap | mqvi.net | kendine ait |
| Kurulum | LiveKit betiği | kurulum betiği |
| Özellikler | 1 GB RAM, 1 çekirdek | 4 GB RAM, 2 vCPU |
| Maliyet | ~$3–5/ay VPS | küçük bir VPS |

## İhtiyacın olanlar

- Bir **Linux sunucusu** (Ubuntu 22.04+ / Debian 12+ önerilir). Hetzner, DigitalOcean ya da Contabo hepsi iyi çalışır.
- Tam sunucu için **alan adı opsiyonel** — alan adı olmadan kurulum betiği ücretsiz bir `sslip.io` hostname kullanır, böylece HTTPS yine çalışır. (Tarayıcılar mikrofon, kamera ve ekran paylaşımını düz HTTP üzerinden engeller, dolayısıyla HTTPS önemli.)

Sonraki iki sayfa her yolu adım adım anlatıyor.
