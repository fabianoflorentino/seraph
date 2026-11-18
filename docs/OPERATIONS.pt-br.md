# Operação

[English](OPERATIONS.md) | Português

## Configuração inicial (uma vez)

1. **Repositório:** crie o repositório público `fabianoflorentino/seraph`.
2. **Pages:** Settings → Pages → Source: *GitHub Actions*.
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

## Comandos locais

```bash
make dev/up      # Hugo + coletor em http://localhost:1314
make dev/logs    # acompanha os dois serviços
make dev/down    # para o ambiente
make build       # bin/seraph com metadados de versão
make validate    # apenas validação do catálogo
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
