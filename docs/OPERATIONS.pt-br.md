# Operação

[English](OPERATIONS.md) | Português

## Configuração inicial (uma vez)

1. **Repositório:** crie o repositório público `fabianoflorentino/seraph`.
2. **Pages:** Settings → Pages → Source: *GitHub Actions*. Defina o domínio
   customizado `seraph.fabianoflorentino.dev` e aponte o CNAME DNS para
   `fabianoflorentino.github.io`. O Hugo publica `site/static/CNAME`.
3. **Permissões do workflow:** Settings → Actions → General → permitir que o
   GitHub Actions crie deployments (padrão para Pages).
4. **Worker na Cloudflare:**
   ```bash
   cd cloudflare
   npx wrangler login
   npx wrangler secret put GITHUB_TOKEN    # PAT fine-grained, Actions: write
   npx wrangler secret put TRIGGER_SECRET  # string aleatória
   npx wrangler deploy
   ```
   O PAT precisa de *Actions: write* apenas neste repositório. Nenhum outro
   escopo.
5. **Verificação:**
   ```bash
   curl https://seraph-dispatch.<account>.workers.dev/health
   curl -X POST https://seraph-dispatch.<account>.workers.dev/trigger \
     -H "Authorization: Bearer $TRIGGER_SECRET"
   ```
   A segunda chamada deve retornar `{"ok":true}` e iniciar uma execução em
   segundos.

## Cadência e frescor

- O cron da Cloudflare dispara a cada cinco minutos; o `dashboard.yml` leva
  alguns minutos, então os dados publicados normalmente têm 5–20 minutos.
- Há um `schedule` do GitHub a cada 15 minutos como fallback se o Worker falhar.
- Push em `main` também atualiza o site, deixando edições de catálogo visíveis
  imediatamente.
- Antes da coleta, o workflow restaura o último `state.json` publicado no Pages.
  O `sync` mescla os pontos (até 288 por serviço, cerca de 24h na cadência de
  cinco minutos) para a disponibilidade sobreviver aos runners efêmeros. Se a
  restauração falhar, a execução começa o histórico do zero.

## Comandos locais

```bash
make dev/up      # Hugo + coletor em http://localhost:1314
make dev/logs    # acompanha os dois serviços
make dev/down    # para o ambiente
make build       # bin/seraph com metadados de versão
make validate    # apenas validação do catálogo
make catalog-inspect CATALOG_TARGET=example.com
make catalog-audit
make sync        # grava site/static/data/state.json
make preview     # compila o Hugo e serve em http://localhost:8000
make test        # race + shuffle
make coverage    # cobertura total
make check       # fmt, vet, lint, test, coverage
```

O ambiente completo está em [DEVELOPMENT.pt-br.md](DEVELOPMENT.pt-br.md). Sem `make`:

```bash
go run ./cmd/seraph validate --catalog catalog/packs
go run ./cmd/seraph sync --catalog catalog/packs
docker compose run --rm --no-deps site hugo --minify --gc
python3 -m http.server -d site/public 8000
```

## Contêiner e implantação interna

A imagem de produção é definida pelo `Dockerfile` da raiz:

- O Hugo compila `site/` com `HUGO_BASEURL` (padrão
  `https://seraph.srectl.fabianoflorentino.dev/`).
- O Go compila um binário estático `seraph` para a plataforma alvo.
- O runtime é `nginx-unprivileged` na porta `8080`; `docker/entrypoint.sh`
  valida o catálogo, inicia o loop do coletor e serve o site.
- O coletor grava em `SERAPH_OUT_DIR` (padrão
  `/usr/share/nginx/html/data`). Monte um volume nesse caminho para manter o
  histórico entre reinícios.

```bash
make docker-build
make docker-run                 # http://localhost:8080
make docker-push IMAGE_TAG=v0.1.0
```

| Variável | Padrão na imagem | Função |
| --- | --- | --- |
| `SERAPH_OUT_DIR` | `/usr/share/nginx/html/data` | Diretório do snapshot. |
| `SERAPH_BASE_URL` | URL interna de `state.json` | URL pública opcional. |
| `SERAPH_SYNC_INTERVAL` | `300` | Segundos entre coletas. |

Endpoints: `/healthz` para o processo e `/data/state.json` para prontidão dos
dados. O site é servido em `/`.

### Workflows de imagem e release

- `.github/workflows/image.yml` publica `edge` em `main` e tags semver em
  `v*`; requer o ambiente `DOCKERHUB` com `DOCKERHUB_USERNAME` e
  `DOCKERHUB_TOKEN` (Read & Write). Sem eles, apenas avisa e ignora.
- `.github/workflows/release.yml` gera arquivos Linux amd64/arm64, checksums e
  a release do GitHub para cada tag `v*`.
- O app GitOps do srectl observa `deploy/projects/seraph`; atualize a tag da
  imagem para promover uma release.

## Configuração

`config/seraph.yml` é opcional; os padrões embutidos valem quando ele não
existe. A precedência é **flag > ambiente > arquivo > padrão**.

| Chave         | Env                  | Padrão      | Observações                     |
| ------------- | -------------------- | ----------- | ------------------------------- |
| `out_dir`     | `SERAPH_OUT_DIR`     | `site/static/data` | Diretório do `state.json`.      |
| `base_url`    | `SERAPH_BASE_URL`    | vazio       | URL pública HTTPS opcional.     |
| `timeout`     | `SERAPH_TIMEOUT`     | `8s`        | Timeout por requisição, 1–60s.  |
| `concurrency` | `SERAPH_CONCURRENCY` | `12`        | Coletas paralelas, 1–128.       |
| `retries`     | `SERAPH_RETRIES`     | `2`         | Tentativas por fonte, 0–10.     |
| `log_level`   | `SERAPH_LOG_LEVEL`   | `info`      | `debug`, `info`, `warn`, `error`. |

`--config` escolhe outro arquivo; `--log-level` sobrescreve o nível configurado.

## Monitoramento

- Saúde do Worker: `GET /health` retorna JSON com `"status":"ok"`.
- Falhas de dispatch aparecem nos logs da Cloudflare (`wrangler tail`).
- Falhas de workflow aparecem em Actions e derrubam o deploy do Pages, mantendo
  a publicação anterior no ar.
- O painel mostra um banner de erro quando não consegue buscar o snapshot e
  mantém os últimos dados renderizados.

## Solução de problemas

| Sintoma                               | Causa provável                                  | Correção                                                 |
| ------------------------------------- | ----------------------------------------------- | -------------------------------------------------------- |
| Snapshot nunca muda                   | Cron do Worker não publicado ou PAT expirado     | `npx wrangler tail`, reemitir PAT, redeploy.             |
| `GitHub dispatch failed: 401/403`     | PAT sem *Actions: write*                         | Atualizar o token fine-grained, `wrangler secret put`.   |
| `GitHub dispatch failed: 404`         | Owner/repo/workflow ou ref errados               | Conferir vars do `wrangler.toml` e o nome do workflow.   |
| `unsupported snapshot schema version` | Versões do site e da CLI divergentes             | Publicar CLI e site do mesmo commit.                     |
| `invalid catalog: ... duplicate slug` | Mesmo slug em dois packs                         | Manter uma definição e referenciar a região correta.     |
| Banner no painel, dados antigos       | Deploy do Pages falhou                           | Ver a última execução em Actions; dados ficam antigos, não somem. |
| Catálogo válido mas card ausente      | Filtro de região ou favoritos                    | Conferir os botões de região e a aba de favoritos.       |
| Card `unknown` com erro               | Fonte inacessível, erro HTTP ou payload inválido | Abrir a status page e conferir o log do workflow para o slug do serviço. |
| `dial tcp 0.0.0.0:443`                | DNS sinkhole/adblock ou DNS dividido no runner  | Liberar o host de status ou usar o host canônico do Statuspage.           |
| Painel de disponibilidade ausente     | Menos de três observações em 24h                 | Aguardar o coletor; o histórico aparece após algumas verificações.       |

## Playbook de incidente

1. Verifique o Worker: `/health` e `wrangler tail`.
2. Verifique o Actions: o `dashboard.yml` foi disparado? Falha em validate, sync
   ou deploy?
3. Se o GitHub estiver degradado, aguarde: o último deploy do Pages continua
   servindo.
4. Se um serviço específico falhar, corrija a entrada no catálogo; os demais não
   são afetados.
5. Registre mudanças visíveis ao usuário no `CHANGELOG.md`.

## Custo

- GitHub Actions e Pages: gratuitos para repositórios públicos dentro dos
  limites padrão.
- Cloudflare Workers: plano gratuito cobre 100 mil requisições/dia; o cron usa
  288 invocações/dia.
- Sem banco, sem object storage, sem monitoramento pago.

## Atualizações

- O Dependabot abre PRs semanais para módulos Go, `wrangler` do Worker e
  Actions. A CI valida.
- Incremente `schema_version` apenas em mudanças incompatíveis do snapshot;
  atualize site e CLI na mesma release.
