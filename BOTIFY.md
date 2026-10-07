# Fork Botify de v8go

Ce dépôt est un instantané de [tommie/v8go](https://github.com/tommie/v8go) (`deps/tommie_sha`)
renommé `github.com/botify-labs/v8go`. Les ajouts Botify sont listés dans `tools/botify-owned.txt` :

- **Bridge précompilé** : les `.cc` ne sont compilés qu'avec `-tags v8go_source`. Les consommateurs
  lient `deps/<os>_<arch>/libv8go.a` (`v8go.lib` sous Windows), produit par `tools/build_bridge.sh`.
  `tools/check_bridge.sh` vérifie qu'il correspond aux sources.
- `cleanup.*` : `Isolate.Cleanup()` / `Context.Cleanup()`. `Isolate.Cleanup()` exécute les tâches
  V8 de l'isolate ; le soak (`bench/soak_test.go`) ne passe que grâce au timer du memory reducer de
  V8 (≥ 8 s) : le soak de 100k itérations doit durer assez longtemps (~20 s) pour qu'il se déclenche.
- **Allocator shim retiré** : les libs V8 de tommie embarquent l'*allocator shim* de PartitionAlloc,
  qui remplacerait `malloc`/`free`/`new`/`delete` pour tout le processus consommateur (+16 à +40 %
  sur les appels unitaires, Task 14). `tools/sync_tommie.sh` retire ses membres des archives V8
  (`llvm-ar d`, format d'archive conservé) à chaque import, puis lance
  `tools/check_no_allocator_shim.sh`, qui échoue s'il reste un membre du shim ou une définition de
  `malloc`/`free`/`new`/`delete` dans une archive V8. Il faut `llvm-ar`/`llvm-ranlib`/`llvm-nm`
  (présents dans l'image Docker).
- **Chemin JS→Go** (Task 25, `bench/results/2026-10-07-callback/callback-perf.md`) :
  `botify_values.h` (valeurs suivies par un contexte : vecteur indexé au lieu de
  l'`unordered_map` de tommie, libérées par adresse de handle décroissante au `Cleanup`),
  `botify_context.h` (le `m_ctx` d'un contexte rangé dans ses *embedder data* : un callback ne
  rappelle plus Go pour le trouver), un callback C++ sans `Locker`/`Isolate::Scope` ni `Global`
  temporaire, et côté Go une seule allocation par appel jusqu'à 4 arguments. Le bridge en dépend :
  un push sur upgrade-v8 qui touche `*.cc`, `*.h`, `tools/patches/` ou `deps/v8_hash` relance
  `botify-bridge` (pas un `.go` à `//export` seul : `check_bridge` le signale, lancer le workflow à la main).
  Les modifications des fichiers de tommie sont des patchs,
  `tools/patches/*.patch`, que `tools/sync_tommie.sh` applique dans l'ordre après l'import : il
  s'arrête si l'un d'eux ne s'applique plus, en laissant appliqués les précédents. Le régénérer
  alors contre cet état : `git add -A` (sans committer), refaire la modification à la main, puis
  `git diff -- <fichiers modifiés> > tools/patches/<patch>`, et relancer l'import.
- `bench/` : comparaison avec `v0.6.0-botify-baseline` (V8 9.0). `tools/docker/` : environnement de dev.
  Si `GONOSUMDB` est défini, il remplace la valeur tirée de `GOPRIVATE` : y inclure `github.com/botify-hq/*` (ex. `GONOSUMDB=github.com/botify-hq/*,github.com/botify-labs/v8go`).

Mettre à jour V8 :
1. lancer `tools/docker/dev.sh 'tools/sync_tommie.sh <sha>'` (ou le workflow `botify-sync-upstream`) ;
2. lancer le workflow `botify-bridge` sur la branche ;
3. vérifier que `botify-ci` passe ;
4. fusionner, puis lancer `tools/pin_deps.sh <sha poussé>`.

Benchmarks : `tools/docker/dev.sh bench/run.sh`. Migration des consommateurs : `MIGRATION.md`.

## Piège clang-format

`clang-format` réécrit la première ligne `//go:build v8go_source` des fichiers `.cc` en
`// go:build v8go_source`. Le tag de build est alors ignoré **sans erreur** : les `.cc` sont compilés
dans le build normal, ou disparaissent du build `-tags v8go_source`. Après tout formatage, vérifier
la ligne 1 de chaque `.cc` (`head -1 *.cc`) : elle doit être exactement `//go:build v8go_source`.
