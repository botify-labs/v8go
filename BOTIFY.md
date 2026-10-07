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

Bridges et pins des modules `deps/*` : un consommateur ne prend pas `deps/<os>_<arch>` dans le commit
de v8go qu'il requiert, mais dans les versions des modules `deps/*` que le `go.mod` de v8go épingle
(MVS). botify-ci vérifie les deux :
- `bridge-fresh` (`tools/check_bridge.sh`) : les bridges de ce commit correspondent aux sources ;
- `pinned-deps-fresh` (`tools/check_pinned_deps.sh`) : les modules `deps/*` épinglés par `go.mod`
  portent un `bridge.sha256` égal à `tools/bridge_hash.sh`, et `bench/go.mod` épingle les mêmes
  versions.

Après toute modification de ce qu'empreinte `tools/bridge_hash.sh` (C++, patchs, `//export`, V8) :
1. pousser. `botify-bridge` reconstruit les bridges (seul pour `*.cc`, `*.h`, `tools/patches/` et
   `deps/v8_hash` sur upgrade-v8, sinon le lancer à la main) et pousse un commit
   « Rebuild prebuilt v8go bridges ». D'ici là, `bridge-fresh` et `pinned-deps-fresh` échouent :
   c'est voulu ;
2. `git pull`, puis `tools/docker/dev.sh 'tools/pin_deps.sh <sha du commit de bridges>'` ; committer
   `go.mod`, `go.sum` et `bench/go.mod`, et pousser ;
3. ce push relance `botify-ci` (le commit du bot n'en déclenche pas) : le commit de pin ne change pas
   le C++, donc `pinned-deps-fresh` passe.

Un pin est obligatoire après chaque reconstruction des bridges, avant de tagger ou de fusionner.

Mettre à jour V8 :
1. lancer `tools/docker/dev.sh 'tools/sync_tommie.sh <sha>'` (ou le workflow `botify-sync-upstream`) ;
2. lancer le workflow `botify-bridge` sur la branche, puis épingler (`tools/pin_deps.sh`, ci-dessus) ;
3. vérifier que `botify-ci` passe, `pinned-deps-fresh` compris ;
4. fusionner par un *merge commit* (pas de squash ni de rebase : `go.mod` référence des commits de
   la branche). Pour une version : tagger les modules `deps/*` sur ce commit, lancer
   `tools/pin_deps.sh <commit tagué>` (qui résout alors les tags), pousser, puis tagger le module racine.

Benchmarks : `tools/docker/dev.sh bench/run.sh`. Migration des consommateurs : `MIGRATION.md`.
Dépendances cgo des consommateurs (liburlnorm, zstd, igzip…) et nouvelles libs : `CGO-DEPENDENCIES.md`.

## Piège clang-format

`clang-format` réécrit la première ligne `//go:build v8go_source` des fichiers `.cc` en
`// go:build v8go_source`. La contrainte de build est alors ignorée **sans erreur** : les `.cc` sont
compilés dans tous les builds, y compris ceux des consommateurs (sans clang ≥ 21 ni `-nostdinc++`,
qui échouent alors). `tools/sync_tommie.sh` place donc la contrainte entre des marqueurs
`// clang-format off` / `// clang-format on`. Après tout formatage, vérifier les lignes 1 à 3 de
chaque `.cc` (`head -3 *.cc`) : la ligne 2 doit être exactement `//go:build v8go_source`.
