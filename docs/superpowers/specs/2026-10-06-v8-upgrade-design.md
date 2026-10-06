# Montée de version V8 + benchmarks — design

Date : 2026-10-06
Statut : en revue (révision 2 : livraison 100 % statique avec bridge précompilé)

## Contexte

- `botify-labs/v8go` est un fork de `rogchap/v8go` (v0.6.0+, V8 **9.0.257.18**, 2021).
- Les consommateurs utilisent
  `replace rogchap.com/v8go => github.com/botify-labs/v8go v0.0.0-20211129082619-6f9829d18985`.
  La **référence de production est donc le commit `6f9829d`** (et non `master`), qui ajoute
  `Isolate.Cleanup()` / `Context.Cleanup()` pour réutiliser un isolate/contexte longue durée.
  gojs les appelle après chaque page (`cdf/gojs/js/v8/v8.go:82`).
- Qui lie réellement V8, vérifié dans les `go.sum` : **ftl** (backend + `backend/apps/cbjsex`),
  **pulse**, **cdf/gojs**. r5 et veribot n'utilisent que des sous-packages de gojs (`stringmatch`),
  sans V8.
- `tommie/v8go` est le fork maintenu : V8 à jour (`deps/v8_hash` = `b3d6849`, soit la V8 de
  Chrome 154, qui embarque **ICU 78.2**), libs précompilées pour linux/darwin amd64+arm64 et
  windows amd64 (ABI MSVC), Intl activé. Chez tommie, les consommateurs compilent `v8go.cc` avec
  clang ≥ 21 et `CGO_CXXFLAGS=-nostdinc++`, des variables d'environnement globales à tout le build.
- gojs lie aussi **liburlnorm** (C++ compilé avec gcc, ICU statique : 65 sous linux et windows,
  71 sous macOS ; libs Windows en MinGW).

## Objectifs

1. Passer à la dernière V8 stable sur linux amd64/arm64, darwin amd64/arm64 et windows amd64.
2. **Tout en statique**, y compris sous Windows (pas de DLL).
3. Ne rien changer à la toolchain des consommateurs sous linux et macOS (gcc / Apple clang, pas de flags).
4. Conserver `Cleanup` sans fuite ni régression.
5. Comparer les performances ancienne/nouvelle version sur v8go et sur gojs.
6. Mettre à jour gojs, ftl et pulse, CI comprise.

## Hors périmètre

- Mise à jour de l'ICU de liburlnorm. Passer en ICU 78 créerait des symboles `_78` en double avec
  l'ICU de V8, et changerait la normalisation des URL en production. C'est un chantier à part,
  à valider sur `liburlnorm/fixtures`.
- Windows en Go < 1.27, et toute approche DLL.
- Nouveaux scénarios gojs à partir de données de prod : on utilise les scénarios existants de gojs.

## Contraintes

- Rien n'est installé sur le poste Windows du développeur : Docker (linux/amd64) ou GitHub Actions.
- Aucun push sans accord explicite.

## Décisions

| Sujet | Décision |
|---|---|
| Base | Instantané de `tommie/v8go`, importé par `tools/sync_tommie.sh`, sans historique ni android |
| Module path | `github.com/botify-labs/v8go`, ainsi que `deps/<os>_<arch>` ; libs copiées dans ce repo (~760 Mo par version) |
| Livraison | **Bridge C++ précompilé** : les `.cc` de v8go sont compilés en CI en `deps/<os>_<arch>/libv8go.a` (`v8go.lib` sous Windows) et liés en statique avec V8. Les consommateurs ne compilent aucun C++ |
| Mode source | Tag `v8go_source` : compile les `.cc` comme chez tommie (clang 21 + `-nostdinc++`), pour développer v8go et lancer LeakSanitizer |
| Linux / macOS | Consommateurs inchangés : gcc ou Apple clang, sans flags (sauf vérification du build statique de pulse) |
| Windows | Statique MSVC : Go ≥ 1.27, `CC="clang -fuse-ld=lld"` (clang ≥ 21 ciblant MSVC). Les packages cgo compilés depuis les sources (DataDog/zstd…) n'ont rien à faire. Seules les libs **précompilées en MinGW** sont recompilées en MSVC (`/MT`) : liburlnorm, plus ce que révélera l'inventaire cgo |
| liburlnorm Windows | Recompilé en MSVC avec **ICU 65** (comme sous linux, donc normalisation identique à la production) |
| i18n | Fourni par tommie (Intl activé) ; la branche PW-1983 n'est pas reportée |
| Cleanup | Réécrit pour le modèle mémoire de tommie |

## 1. Resynchronisation du fork

- Branche `upgrade-v8`, créée depuis `master`. `tools/sync_tommie.sh <sha>` importe l'instantané,
  renomme le module, retire android et les workflows de build V8 de tommie, puis :
  - ajoute `//go:build v8go_source` en tête de chaque `.cc` importé, entre `// clang-format off` et `// clang-format on` (clang-format réécrit sinon le tag en `// go:build`) ;
  - ajoute aux `deps/*/cgo.go` les lignes `#cgo !v8go_source LDFLAGS: -lv8go`
    (et `-lm` sous linux, car gcc ne lie pas libm d'office).
- Les fichiers Botify (listés dans `tools/botify-owned.txt`) sont préservés.
- Tag `v0.6.0-botify-baseline` posé sur `6f9829d`. Ajout d'un `.gitattributes` (LF).
- `botify-sync-upstream.yml` (hebdomadaire) importe le nouvel instantané et ouvre une PR.
  `botify-bridge.yml` reconstruit les 5 bridges.
- Après le push, `tools/pin_deps.sh` épingle les modules `deps/*` sur notre commit.

## 2. Bridge précompilé

- `tools/build_bridge.sh`, exécuté sur la plateforme cible :
  1. `go tool cgo` génère `_cgo_export.h` (les callbacks Go appelés par le C++) ;
  2. compilation de chaque `.cc` avec les `CgoCXXFLAGS` du package (`go list`), `-nostdinc++` et `-O2` ;
  3. archivage avec `ar` (`llvm-lib` sous Windows), puis écriture de
     `deps/<os>_<arch>/bridge.sha256`, une empreinte des sources (`*.cc`, `*.h`, fichiers Go
     contenant des `//export`, `cgo.go`, `deps/v8_hash`).
- La CI échoue si l'empreinte d'une plateforme ne correspond plus aux sources (bridge périmé).
- En local, Docker produit `linux_amd64`. La CI (runners natifs) produit les 4 autres.

## 3. Report de `Cleanup`

API inchangée : `func (i *Isolate) Cleanup()`, `func (c *Context) Cleanup()`. Code dans
`cleanup.h`, `cleanup.cc` (avec `//go:build v8go_source`), `cleanup.go` et `cleanup_test.go`.

- `ContextCleanup` libère toutes les entrées de `ctx->vals` **sauf** les propriétaires faibles des
  valeurs Go (`go_handle != 0`, créés par `NewValueGo`). JS peut encore référencer leur `External`,
  et `GoValueWeakCallback` libère l'entrée et son `cgo.Handle` quand V8 collecte l'objet. Il libère
  aussi les `unboundScripts`.
- `Isolate.Cleanup` applique la même chose au contexte interne, puis recrée `null` et `undefined`.
- Après `Cleanup`, toute `*Value` obtenue avant l'appel est invalide (déjà le cas dans `6f9829d`).
- Tests : nombre de valeurs retenues revenu à sa base ; `cgo.Handle` libéré (finaliseur Go) une
  fois la valeur Go collectée par V8 ; valeur Go encore référencée par JS toujours utilisable ;
  unbound scripts et cache de code ; `TerminateExecution` ; callbacks et promesses ; appels
  répétés ; appel après `Close`/`Dispose` sans effet.

## 4. Benchmarks

- Module `bench/` : un shim (build tag `v8baseline` ou non) donne les mêmes noms aux deux versions.
  Suites : binding (création, appels Go↔JS, valeurs, JSON, Cleanup), JS pur (regex, objets,
  tableaux, chaînes, JSON), compilation (lodash à froid, avec cache, démarrage d'un worker) et
  mémoire (heap V8 par isolate et par contexte).
- gojs : `benchmark_test.go` existant (`BenchmarkV8_*`), lancé sur cdf master + `6f9829d` et
  sur la branche `upgrade-v8go`.
- `bench/run.sh` (Docker) : `-count=10 -benchmem`, `benchstat`, `summary.md`. La nouvelle version
  est compilée **comme chez un consommateur** (gcc + bridge précompilé).
- `botify-bench.yml` : comparaison sur darwin amd64/arm64 ; nouvelle version seule sur linux arm64 et windows.

## 5. Non-régression et fuites

- Tests v8go sur les 5 cibles, en mode consommateur avec `-race` et en mode source.
- Tests de la référence disparus portés dans `compat_test.go` ; `apidiff` documenté dans `MIGRATION.md`.
- LeakSanitizer en mode source (CI + Docker).
- Endurance v8go et gojs : 100 000 cycles avec `Cleanup` (RSS, heap V8, heap Go, goroutines),
  sur les deux versions.
- Compatibilité d'édition de liens, à vérifier tôt :
  - gojs + liburlnorm (libstdc++ + ICU 65) avec le bridge (libc++ de Chromium + ICU 78) ;
  - lien entièrement statique façon pulse (`-extldflags '-static -lm'`) ;
  - exécution dans Amazon Linux 2023 (cbjsex).

## 6. Consommateurs

- **Inventaire cgo** de ftl et pulse (`go list -deps`, GOOS=linux et windows) : liste des
  packages cgo et de leurs libs précompilées, dans `docs/superpowers/cgo-inventory.md`.
- **cdf** : branche `upgrade-v8go`. gojs passe sur `github.com/botify-labs/v8go`, liburlnorm reçoit
  ses libs Windows MSVC (`lib/windows_msvc`, construites par un workflow Windows), et `go-ci.yaml`
  gagne un job Windows pour gojs.
- **ftl** et **pulse** : branches qui montent la version de gojs et retirent le `replace rogchap`.
  Leur CI linux doit passer sans changement de toolchain (`build-backend`, `ftl-back-test`,
  `ftl-back-integration-test`, `cbjsex` dans AL2023, `go-backend` statique, `go-lambda`).

## 7. Critères d'acceptation

- Tests v8go verts sur les 5 cibles ; bridges à jour (empreintes conformes).
- LeakSanitizer : aucune fuite attribuée à v8go.
- Endurance : RSS en hausse de moins de 5 % sur la seconde moitié, nombre de goroutines stable,
  et la nouvelle version ne fait pas pire que la référence.
- `benchstat` : aucune régression significative de plus de 5 % sans explication dans `summary.md`.
- gojs (cdf), ftl et pulse : CI verte sur les branches de mise à jour ; gojs vert sous Windows (MSVC).
- `MIGRATION.md` complet.

## 8. Risques

| Risque | Mitigation |
|---|---|
| Deux runtimes C++ (libc++abi de Chromium et libstdc++ de liburlnorm), surtout en lien entièrement statique (pulse) : symboles `__cxa_*` en double | Test fait en premier (lien dynamique et `-static`) ; si ça échoue, arrêt et révision du design |
| glibc des libs V8 trop récente pour AL2023 | Test dans un conteneur `amazonlinux:2023` |
| `Cleanup` incompatible avec les propriétaires faibles | Tests dédiés, LeakSanitizer, endurance |
| Bridge périmé après une synchronisation | Empreinte par plateforme vérifiée en CI ; `botify-bridge.yml` |
| Autres libs MinGW précompilées dans ftl/pulse | Inventaire cgo, puis recompilation MSVC de chacune |
| Bruit de mesure dans Docker | `-count=10` + benchstat ; mêmes conditions pour les deux versions |
