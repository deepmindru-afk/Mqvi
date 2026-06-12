# Sadece ses sunucusu (LiveKit)

mqvi.net hesabını normal şekilde kullan — sadece **ses ve görüntü** kendi LiveKit sunucundan geçsin.

## Linux

Sunucuna SSH ile bağlan ve şunu çalıştır:

```bash
curl -fsSL https://raw.githubusercontent.com/akinalpfdn/Mqvi/main/deploy/livekit-setup.sh | sudo bash
```

Betik LiveKit'i indirir, güvenlik duvarı portlarını açar, güvenli kimlik bilgileri üretir, `livekit.yaml` dosyasını yazar ve onu bir systemd servisi olarak başlatır.

**Gereksinimler:** herhangi bir Linux sunucusu (Ubuntu 22.04+ / Debian 12+), 1 GB RAM, 1 CPU çekirdeği.

## Windows

**PowerShell'i Yönetici olarak** aç ve şunu çalıştır:

```powershell
irm https://raw.githubusercontent.com/akinalpfdn/Mqvi/main/deploy/livekit-setup.ps1 | iex
```

Yukarıdakiyle aynı, ek olarak router portlarını UPnP üzerinden yönlendirmeyi dener ve LiveKit'i açılışta otomatik başlatacak şekilde ayarlar.

**Gereksinimler:** Windows 10/11. Kendi PC'in ise açık ve çevrimiçi kalmalı.

## mqvi'ye bağla

Betik bittiğinde **3 değer** yazdırır — bir **URL**, **API key** ve **API secret**. mqvi'de yeni bir sunucu oluştur, **Self-Hosted** seç ve bunları yapıştır. Hepsi bu.

![3 LiveKit değeriyle self-hosted sunucu formu](assets/voice-livekit-1.png)

## Sorun giderme

| Sorun | Çözüm |
| --- | --- |
| Ses bağlanmıyor | Muhtemelen portlar kapalı — hem güvenlik duvarını **hem de** bulut sağlayıcının web güvenlik duvarını kontrol et |
| Bağlandı ama ses yok | UDP portları **50000–60000** engellenmiş olabilir; oradaki UDP'ye izin ver |
| "Connection refused" | LiveKit çalışmıyor — `systemctl status livekit` (Linux) |
| LAN'da çalışıyor ama internette çalışmıyor | `livekit.yaml` içinde `use_external_ip: true` ayarla; 7880, 7881, 7882 ve 50000–60000 portlarını yönlendir |
