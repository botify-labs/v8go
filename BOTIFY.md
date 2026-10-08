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
- **Runtime C++ de V8 isolé (Linux)** : libc++abi et libc++ de Chromium définissent la couche ABI C++
  (`__cxa_*`, `__gxx_personality_v0`, `std::exception` et les autres exceptions `std::`, leurs
  typeinfo et vtables, `__dynamic_cast`, `std::terminate`, `operator new`/`delete`…) sous les mêmes
  noms que libstdc++/libsupc++. Sans isolation, un binaire entièrement statique qui lie aussi du C++
  g++ échoue (`multiple definition`), et en dynamique le runtime de V8 supplante celui de
  `libstdc++.so` (une exception levée dans libstdc++, `std::stoi` par exemple, finit en
  `std::terminate`). Après le retrait du shim, `tools/sync_tommie.sh` :
  1. génère `tools/cxx-runtime-rename.map` avec `tools/gen_cxx_runtime_rename_map.sh` : tout symbole
     global défini par `libc++-cr.a`/`libc++abi-cr.a` hors des namespaces privés `__Cr` et
     `__llvm_libc_cr` (499 symboles), qui reçoit le suffixe `.v8cr` (`c++filt` affiche
     `typeinfo for std::exception [clone .v8cr]`) ;
  2. l'applique avec `tools/rename_cxx_runtime.sh` (`objcopy --redefine-syms`, idempotent) aux
     archives V8, libc++, compiler-rt et aux bridges de `deps/linux_*`, définitions et références :
     le groupe COMDAT `DW.ref.__gxx_personality_v0`, par lequel les CIE de `.eh_frame` atteignent la
     routine de personnalité, est renommé aussi. `_Unwind_*` (l'unwinder de libgcc, partagé) et la
     libc ne sont pas touchés ;
  3. termine par `tools/check_cxx_runtime_isolated.sh`, qui échoue si une archive Linux définit ou
     référence encore un nom d'origine, ou si la table et les alias ne sont pas ceux que le script
     génère.

  `tools/build_bridge.sh` renomme de même les bridges Linux, et la table et les deux
  scripts (`build_bridge.sh`, `rename_cxx_runtime.sh`) entrent dans `tools/bridge_hash.sh`. Mode source : les objets compilés par cgo référencent les noms d'origine ;
  `deps/linux_*/libv8go_cxxalias.a` (un script d'édition de liens généré, lié avant les archives en
  mode source seulement) les définit comme alias des noms renommés (`EXTERN` + `PROVIDE`), et
  `botify_cxxalias_linux.go` redirige `operator new`/`delete` par `--wrap`, pour qu'ils restent ceux
  de V8 même à côté de ceux d'ASan (`-tags leakcheck`). Preuve : `internal/cxxprobe` (C++ g++ lié avec
  v8go, tag `cxxprobe`), exécuté en lien entièrement statique par le job `static-cxx-probe`.
- **Limites du mode source** (`-tags v8go_source`, réservé au développement de v8go ; le mode
  consommateur, par défaut, est isolé) : l'isolation du runtime C++ de V8 n'y existe pas, l'exécutable
  définit et exporte toujours les noms d'origine (`__cxa_*`, `std::exception`…), qu'il fait passer
  devant ceux de libstdc++.so.
  - Du code g++/libstdc++ lié avec `-tags v8go_source` n'est **pas supporté** : ses exceptions et sa
    RTTI se lient au runtime de Chromium (lien dynamique : `std::terminate` ; lien statique :
    `multiple definition`) ;
  - sous ASan (`-tags leakcheck`), `operator new`/`delete` de V8 et de v8go passent par ceux de
    libc++abi : la détection des `new`/`delete` non appariés (`alloc-dealloc-mismatch`) est perdue
    pour ce code. LeakSanitizer et les contrôles au niveau de `malloc` ne changent pas ;
  - les alias (`libv8go_cxxalias.a`) ne sont testés qu'avec GNU ld (le défaut du pilote clang-21) ;
    lld et gold ne le sont pas.
- **Chemin JS→Go** (Task 25, `bench/results/2026-10-07-callback/callback-perf.md`) :
  `botify_values.h` (valeurs suivies par un contexte : vecteur indexé au lieu de
  l'`unordered_map` de tommie, libérées par adresse de handle décroissante au `Cleanup`),
  `botify_context.h` (le `m_ctx` d'un contexte rangé dans ses *embedder data* : un callback ne
  rappelle plus Go pour le trouver), un callback C++ sans `Locker`/`Isolate::Scope` ni `Global`
  temporaire, et côté Go une seule allocation par appel jusqu'à 4 arguments. Le bridge en dépend :
  un push sur une branche autre que master (upgrade-v8 jusqu'à sa fusion, puis la branche de la PR)
  qui touche `*.cc`, `*.h`, `tools/patches/`, `deps/v8_hash` ou la table de
  renommage du runtime C++ relance
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
  le même job vérifie l'absence du shim et l'isolation du runtime C++ (`tools/check_cxx_runtime_isolated.sh`) ;
- `pinned-deps-fresh` (`tools/check_pinned_deps.sh`) : les modules `deps/*` épinglés par `go.mod`
  portent un `bridge.sha256` égal à `tools/bridge_hash.sh`, et `bench/go.mod` épingle les mêmes
  versions.

Après toute modification de ce qu'empreinte `tools/bridge_hash.sh` (C++, patchs, `//export`, V8,
table de renommage du runtime C++, scripts `build_bridge.sh` et `rename_cxx_runtime.sh`) :
1. pousser. `botify-bridge` reconstruit les bridges (seul pour `*.cc`, `*.h`, `tools/patches/`,
   `tools/cxx-runtime-rename.map`, ses scripts et
   `deps/v8_hash` sur une branche autre que master, sinon le lancer à la main) et, si les bridges
   ne correspondent plus aux sources, pousse sur cette branche un commit
   « Rebuild prebuilt v8go bridges » (jamais sur master, protégée : passer par une PR). D'ici là,
   `bridge-fresh` et `pinned-deps-fresh` échouent : c'est voulu ;
2. `git pull`, puis `tools/docker/dev.sh 'tools/pin_deps.sh <sha du commit de bridges>'` ; committer
   `go.mod`, `go.sum` et `bench/go.mod`, et pousser ;
3. ce push relance `botify-ci` (le commit du bot n'en déclenche pas) : le commit de pin ne change pas
   le C++, donc `pinned-deps-fresh` passe.

Un pin est obligatoire après chaque reconstruction des bridges, avant de tagger ou de fusionner.

Mettre à jour V8 :
1. lancer `tools/docker/dev.sh 'tools/sync_tommie.sh <sha>'` (ou le workflow `botify-sync-upstream`) ; il
   retire le shim, renomme le runtime C++ des archives Linux (table régénérée : committer son diff avec
   les archives) et finit par `tools/check_cxx_runtime_isolated.sh` ;
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
