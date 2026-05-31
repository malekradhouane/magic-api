# Déploiement MAGIC — Guide pas à pas

Guide complet pour mettre MAGIC en production de façon **fiable et sécurisée**, à
moindre coût, pour une boutique e-commerce (clientèle Tunisie).

## Architecture

```
                 Cloudflare (gratuit)
        DNS · CDN · DDoS · WAF · SSL
        ┌──────────────┬───────────────┬─────────────────┐
        │              │               │                 │
  votredomaine.tn  admin.…tn      api.…tn            cdn.…tn
        │              │               │                 │
 Cloudflare Pages  CF Pages      VPS Hetzner       Cloudflare R2
  (magic-front)   (magic-admin)  Nginx→API Go      (images produits)
                                 + PostgreSQL
                                 (Docker)
```

- **Fronts statiques** (`nuxt generate`) → Cloudflare Pages : gratuit, CDN mondial
  (POP à Tunis), encaisse le gros du trafic sans toucher le serveur.
- **API Go + PostgreSQL** → 1 petit VPS Hetzner (**CX22**, ~5 €/mois) : suffisant
  pour 1000 produits et une clientèle locale.
- **Images** → Cloudflare R2 (gratuit jusqu'à 10 Go, pas de frais de sortie).

### Pourquoi c'est robuste / sûr
- L'IP du VPS est **cachée** derrière Cloudflare ; 80/443 n'acceptent que les IP
  Cloudflare (script `02-firewall-cloudflare.sh`).
- PostgreSQL **jamais exposé** à Internet (réseau Docker interne).
- SSH par clé uniquement, `fail2ban`, `ufw`, mises à jour auto.
- Cloudflare absorbe les DDoS et filtre les attaques applicatives (WAF).

---

## 🚀 Démarrer SANS domaine (phase IP — test/validation)

Tu n'as pas encore de domaine ? Tu peux commencer dès maintenant avec **juste
l'IP du VPS** pour valider que tout fonctionne, puis brancher le domaine après.

### Ce qui marche / ce qui attend
| | Phase IP (maintenant) | Avec domaine (plus tard) |
|---|---|---|
| API Go + PostgreSQL | ✅ `http://<IP>:5002` | ✅ `https://api.domaine.tn` |
| Migrations DB, stock, commandes | ✅ | ✅ |
| HTTPS / SSL | ❌ (HTTP en clair) | ✅ Let's Encrypt + Cloudflare |
| Cloudflare (DDoS/WAF/IP cachée) | ❌ | ✅ |
| Fronts | en local, **ou Netlify** | Netlify / Cloudflare Pages + domaine |

> ⚠️ La phase IP sert à **tester**, pas à ouvrir la boutique au public
> (pas de HTTPS = navigateurs "Non sécurisé", login/paiement en clair).
> Mets le domaine **avant** l'ouverture réelle.

### Fichiers dédiés à la phase IP
- `docker-compose.ip.yml` — API exposée en HTTP sur le VPS (port 5002)
- `.env.ip.example` — variables avec `http://<IP>` au lieu du domaine
- `nginx/magic-api-ip.conf` — (optionnel) proxy HTTP port 80

### Procédure phase IP
1. **Crée et sécurise le VPS** : suis *Étape 1* et *Étape 2* ci-dessous
   (Hetzner + `01-server-setup.sh`). Tout ça est identique avec ou sans domaine.
2. **Ouvre le port de l'API dans le firewall** (juste pour la phase test) :
   ```bash
   sudo ufw allow 5002/tcp
   ```
3. **Lance l'API + DB** :
   ```bash
   git clone <ton-repo> ~/MAGIC
   cd ~/MAGIC/magic-api/deploy
   cp .env.ip.example .env.ip
   nano .env.ip            # remplir secrets + remplacer <IP> par l'IP du VPS
   docker compose -f docker-compose.ip.yml --env-file .env.ip up -d --build
   docker compose -f docker-compose.ip.yml --env-file .env.ip logs -f magic-api
   ```
4. **Teste** depuis ton PC : `curl http://<IP>:5002/health` (ou un endpoint connu).

### Tester les fronts pendant la phase IP

**Option A — Fronts en local (le plus simple pour "juste tester")**
Sur ton PC, pointe les fronts vers l'API du VPS et lance-les :
```bash
# magic-front
cd magic-front
echo "NUXT_PUBLIC_API_BASE_URL=http://<IP>:5002" > .env
npm install && npm run dev          # http://localhost:3000

# magic-admin (autre terminal)
cd magic-admin
echo "NUXT_PUBLIC_API_BASE_URL=http://<IP>:5002" > .env
npm install && npm run dev          # http://localhost:3001
```
Pas de problème de "mixed content" car localhost est traité comme sécurisé.

**Option B — Fronts sur Netlify (sous-domaine HTTPS gratuit)**
Netlify héberge tes fronts statiques gratuitement avec une URL
`https://xxx.netlify.app` en HTTPS — **sans que tu aies de domaine**.
- New site → import depuis GitHub
- magic-front : *Base directory* `magic-front`, *Build command* `npm run generate`,
  *Publish directory* `magic-front/.output/public`
- magic-admin : idem avec `magic-admin`
- Variables d'env Netlify : `NUXT_PUBLIC_API_BASE_URL`, `NUXT_PUBLIC_R2_BASE_URL`,
  `NODE_VERSION=20`

> ⚠️ **Piège HTTPS→HTTP** : un front Netlify (HTTPS) qui appelle une API en
> `http://<IP>` sera **bloqué par le navigateur** (mixed content). Pour utiliser
> Netlify réellement, il faut une API en HTTPS → c'est le moment de prendre le
> domaine + Cloudflare. Pour du pur test, préfère l'**Option A (local)**.

### Passer de la phase IP au domaine (plus tard)
Quand tu achètes le domaine, la migration est rapide :
1. Ajoute le domaine dans **Cloudflare**, crée l'enregistrement `A api → <IP>` (proxy ON).
2. Sur le VPS :
   ```bash
   sudo ufw delete allow 5002/tcp          # referme le port de test
   docker compose -f docker-compose.ip.yml --env-file .env.ip down
   cp .env.ip .env.prod                    # puis adapte BASE_URL/CORS en https://…
   nano .env.prod
   docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
   ```
3. Mets en place **Nginx + HTTPS** (*Étape 5*) puis **verrouille sur Cloudflare**
   (*Étape 6*).
4. Branche ton domaine custom sur Netlify (ou bascule vers Cloudflare Pages) et
   mets à jour `CORS_ALLOWED_ORIGINS` avec les URLs finales.

Le reste du guide ci-dessous décrit la configuration **finale avec domaine**.

---

## Pré-requis
- Un domaine (ex. `votredomaine.tn`) — tu l'as déjà.
- Un compte **Cloudflare** (gratuit) + le domaine ajouté dans Cloudflare
  (changer les nameservers chez ton registrar pour ceux de Cloudflare).
- Un compte **Hetzner Cloud**.
- Le code sur **GitHub** (nécessaire pour Cloudflare Pages).

### Sous-domaines à prévoir (DNS Cloudflare, tous en proxy "orange cloud")
| Nom | Type | Cible | Usage |
|-----|------|-------|-------|
| `votredomaine.tn` | Pages | (auto) | Boutique |
| `admin` | Pages | (auto) | Admin |
| `api` | A | IP du VPS | API Go |
| `cdn` | CNAME | domaine R2 public | Images |

---

## Étape 1 — Créer le VPS Hetzner
1. Hetzner Cloud → **New Server**.
2. Image : **Ubuntu 24.04**. Type : **CX22** (2 vCPU / 4 Go). Localisation :
   Nuremberg ou Falkenstein (bonne latence vers la Tunisie via le CDN).
3. Ajoute ta **clé SSH** (génère-la en local si besoin :
   `ssh-keygen -t ed25519 -C "toi@host"`).
4. (Optionnel +~1 €/mois mais recommandé client sérieux) active les **Backups**.
5. Crée le serveur, note son **IP publique**.

## Étape 2 — Sécuriser le serveur
En local, connecte-toi : `ssh root@<IP>`. Puis copie les scripts sur le serveur
(ou clone le repo). Édite `deploy/scripts/01-server-setup.sh` :
- `NEW_USER` (ex. `magic`)
- `SSH_PUBKEY` = ta clé **publique** (`cat ~/.ssh/id_ed25519.pub`)

Lance :
```bash
sudo bash deploy/scripts/01-server-setup.sh
```
Ça crée l'utilisateur, durcit SSH (clé only), active UFW + fail2ban + updates
auto, et installe Docker.

**Important** : ouvre un NOUVEAU terminal et vérifie
`ssh magic@<IP>` AVANT de fermer la session root.

## Étape 3 — DNS Cloudflare pour l'API
Dans Cloudflare → DNS → ajoute :
- `A` `api` → `<IP du VPS>` — **Proxy activé (orange cloud)**.

SSL/TLS → mode **Full (strict)**.

## Étape 4 — Déployer l'API + PostgreSQL
Sur le serveur (utilisateur `magic`) :
```bash
git clone <ton-repo> ~/MAGIC
cd ~/MAGIC/deploy
cp .env.prod.example .env.prod
nano .env.prod          # remplir TOUS les secrets (voir ci-dessous)
```
Génère des secrets forts :
```bash
openssl rand -base64 48   # MAGIC_CUSTOMER_SECRET
openssl rand -base64 32   # GOTH_SESSION_KEY
openssl rand -base64 32   # POSTGRES_PASSWORD
```
Construis/lance la stack (les **migrations** tournent automatiquement au démarrage
via `storeinit`, voir `magic.yaml › startup.execPre`) :
```bash
# Si tu builds localement, dé-commente la section build: dans le compose
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
docker compose -f docker-compose.prod.yml --env-file .env.prod logs -f magic-api
```
L'API écoute sur `127.0.0.1:5002` (jamais exposée directement).

## Étape 5 — Nginx + HTTPS sur le VPS
```bash
sudo apt-get install -y nginx certbot python3-certbot-nginx
sudo cp ~/MAGIC/deploy/nginx/magic-api.conf /etc/nginx/sites-available/
# remplace api.votredomaine.tn par ton vrai sous-domaine dans le fichier
sudo nano /etc/nginx/sites-available/magic-api.conf
sudo ln -s /etc/nginx/sites-available/magic-api.conf /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo mkdir -p /var/www/certbot
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d api.votredomaine.tn   # certificat d'origine
```
Teste : `curl https://api.votredomaine.tn/health` (ou un endpoint connu).

## Étape 6 — Verrouiller 80/443 sur Cloudflare uniquement
Une fois que l'API répond via Cloudflare :
```bash
sudo bash ~/MAGIC/deploy/scripts/02-firewall-cloudflare.sh
sudo ufw status numbered
```
Désormais, seul le trafic Cloudflare atteint le serveur.

## Étape 7 — Stockage images : Cloudflare R2
1. Cloudflare → R2 → crée le bucket `magic-products`.
2. Crée un **API Token R2** (Access Key / Secret).
3. Connecte un domaine public au bucket → `cdn.votredomaine.tn`.
4. Reporte les valeurs dans `.env.prod` (`R2_*` + `R2_PUBLIC_BASE_URL`) puis
   `docker compose ... up -d` pour recharger l'API.

## Étape 8 — Déployer les fronts sur Cloudflare Pages
Pour **chacun** des deux projets (`magic-front`, puis `magic-admin`) :

Cloudflare → **Workers & Pages** → Create → Pages → Connect to Git → choisis le repo.

**magic-front (boutique)**
| Réglage | Valeur |
|---------|--------|
| Root directory | `magic-front` |
| Build command | `npm run generate` |
| Output directory | `.output/public` |
| Variable env | `NUXT_PUBLIC_API_BASE_URL=https://api.votredomaine.tn` |
| Variable env | `NUXT_PUBLIC_R2_BASE_URL=https://cdn.votredomaine.tn` |
| Variable env | `NODE_VERSION=20` |
| Domaine custom | `votredomaine.tn` |

**magic-admin**
| Réglage | Valeur |
|---------|--------|
| Root directory | `magic-admin` |
| Build command | `npm run generate` |
| Output directory | `.output/public` |
| Variable env | `NUXT_PUBLIC_API_BASE_URL=https://api.votredomaine.tn` |
| Variable env | `NUXT_PUBLIC_R2_BASE_URL=https://cdn.votredomaine.tn` |
| Variable env | `NUXT_IMAGE_PROVIDER` | (selon ta config front) |
| Variable env | `NODE_VERSION=20` |
| Domaine custom | `admin.votredomaine.tn` |

À chaque `git push`, Cloudflare rebuild et redéploie automatiquement.

## Étape 9 — CORS
Dans `.env.prod`, `CORS_ALLOWED_ORIGINS` doit lister exactement :
```
https://votredomaine.tn,https://admin.votredomaine.tn
```
Relance l'API après modification.

## Étape 10 — Sauvegardes automatiques
```bash
chmod +x ~/MAGIC/deploy/scripts/backup-db.sh
crontab -e
# Ajoute :
30 3 * * * /home/magic/MAGIC/deploy/scripts/backup-db.sh >> /home/magic/backups/backup.log 2>&1
```
Garde 14 jours de dumps. Restauration :
```bash
gunzip -c ~/backups/magicdb-XXXX.sql.gz | \
  docker exec -i magic-postgres psql -U <POSTGRES_USER> -d <POSTGRES_DB>
```

---

## Mises à jour applicatives
```bash
cd ~/MAGIC && git pull
cd deploy
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
```
Les fronts se mettent à jour seuls au `git push` (Cloudflare Pages).

## Checklist sécurité (à valider)
- [ ] `.env`, `.env.prod` **jamais** commités (voir avertissement ci-dessous)
- [ ] SSH par clé uniquement, root désactivé, fail2ban actif
- [ ] UFW : 22 ouvert ; 80/443 limités à Cloudflare ; Postgres non exposé
- [ ] Cloudflare SSL = Full (strict), proxy orange sur tous les enregistrements
- [ ] Secrets forts (DB, JWT, session) générés aléatoirement
- [ ] Backups quotidiens + (option) snapshots Hetzner
- [ ] WAF Cloudflare activé (managed rules)

> ### ⚠️ Avertissement sécurité important
> Le fichier `magic-api/.env` du repo contient actuellement des **secrets réels
> en clair** (clés Mailjet, Google OAuth, tokens API, mots de passe). Avant toute
> mise en production :
> 1. **Révoque/régénère** toutes ces clés (elles sont compromises dès qu'elles
>    sont dans l'historique git).
> 2. Retire `.env` du suivi git : `git rm --cached magic-api/.env` et ajoute-le
>    au `.gitignore`.
> 3. Utilise uniquement `deploy/.env.prod` (non commité) en production.

## Coût mensuel estimé
| Poste | Coût |
|-------|------|
| VPS Hetzner CX22 | ~5 € |
| Backups Hetzner (option) | ~1 € |
| Cloudflare (DNS/CDN/WAF/Pages) | 0 € |
| Cloudflare R2 (<10 Go) | 0 € |
| **Total** | **~5-6 €/mois** |
