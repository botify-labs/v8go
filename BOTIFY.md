# Fork Botify de v8go

Ce dépôt est un instantané de [tommie/v8go](https://github.com/tommie/v8go) (`deps/tommie_sha`)
renommé `github.com/botify-labs/v8go`. Les ajouts Botify sont listés dans `tools/botify-owned.txt` :

- **Bridge précompilé** : les `.cc` ne sont compilés qu'avec `-tags v8go_source`. Les consommateurs
  lient `deps/<os>_<arch>/libv8go.a` (`v8go.lib` sous Windows), produit par `tools/build_bridge.sh`.
  `tools/check_bridge.sh` vérifie qu'il correspond aux sources.
- `cleanup.*` : `Isolate.Cleanup()` / `Context.Cleanup()`. `Isolate.Cleanup()` exécute les tâches
  que V8 a postées pour l'isolate (GC, memory reducer, tâches qui règlent des promesses : compilation
  wasm, `Atomics.waitAsync`), sans jamais exécuter de JS ni de callback Go : chaque tâche est
  précédée d'une demande de terminaison, le JS qu'elle lancerait (callbacks `FinalizationRegistry`)
  est interrompu avant sa première instruction, et pendant la pompe (`botify_cleanup_isolate`,
  `botify_context.h`) les callbacks de fonction renvoient `undefined` sans appeler Go, le
  `PromiseRejectedCallback` et les messages console de l'inspecteur ne font rien (patchs 0005 et
  0006). Un checkpoint de microtâches, terminaison demandée, vide ensuite la file : les réactions
  des promesses que les tâches ont réglées (ou qu'un script terminé a laissées) ne s'exécutent pas
  au script suivant. La terminaison est enfin annulée (un `TerminateExecution` demandé pendant
  `Cleanup` l'est aussi), et l'état de limite de heap remis à zéro (patchs 0007 et 0009 :
  `Isolate.HeapLimitReached` avant la pompe, pour qu'une limite atteinte par un GC de la pompe reste
  visible, la terminaison à signaler après). Chaque tâche `FinalizationRegistry` pompée n'élimine
  qu'une cellule morte : au plus 10 000 par `Cleanup` (environ 8 ms). Les deux `Cleanup` ne font rien
  s'ils sont appelés avec du JS de l'isolate sur la pile du thread appelant (depuis un
  `FunctionCallback`) ; depuis une autre goroutine, ils attendent le `Locker`. Le RSS d'un isolate
  long-lived ne plafonne que grâce au timer du memory reducer de V8 (≥ 8 s) : un soak
  (`bench/soak_test.go`) doit durer assez longtemps pour qu'il se déclenche. Le job `soak` de
  botify-ci le fait tourner 25 s (`SOAK_DURATION=25s`, une durée plutôt qu'un nombre d'itérations : 30 000
  itérations ne duraient que 6,8 s sur un runner rapide, avant le timer) et vérifie une croissance
  du RSS ≤ 5 % sur la seconde moitié et un nombre de goroutines stable. Sans `SOAK_DURATION`, le soak
  fait `SOAK_ITERATIONS` itérations (100 000 par défaut, ~20 s ; `bench/run.sh soak`).
- **Allocator shim retiré** : les libs V8 de tommie embarquent l'*allocator shim* de PartitionAlloc,
  qui remplacerait `malloc`/`free`/`new`/`delete` pour tout le processus consommateur (+16 à +40 %
  sur les appels unitaires, `bench/results/2026-10-06-noshim/summary.md`). `tools/sync_tommie.sh` retire ses membres des archives V8
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
- **Chemin JS→Go** (`bench/results/2026-10-07-callback/callback-perf.md`) :
  `botify_values.h` (valeurs suivies par un contexte : vecteur indexé au lieu de
  l'`unordered_map` de tommie, libérées par adresse de handle décroissante au `Cleanup`),
  `botify_context.h` (le `m_ctx` d'un contexte rangé dans ses *embedder data* : un callback ne
  rappelle plus Go pour le trouver), un callback C++ sans `Locker`/`Isolate::Scope` ni `Global`
  temporaire, et côté Go une seule allocation par appel jusqu'à 4 arguments.
- **Garde-fous des callbacks** (`tools/patches/0005-callback-guards.patch`) : une fonction dont le
  `Context` est fermé lève `Error: v8go: context closed` au lieu de déréférencer un contexte nul ; un
  callback Go qui renvoie l'erreur d'un script imbriqué terminé laisse la terminaison se propager au
  lieu de la relancer comme exception attrapable ; pendant `Isolate.Cleanup`, les callbacks renvoient
  `undefined` sans appeler Go.
- **Pompe de `Cleanup` et limite de heap** (`tools/patches/0006-cleanup-pump-guards.patch`,
  `tools/patches/0007-heap-limit.patch`) : pendant `Isolate.Cleanup`, le `PromiseRejectedCallback`
  et les messages console de l'inspecteur n'appellent pas Go. La limite de heap atteinte par un
  script imbriqué (lancé par un callback Go) est signalée (`ErrHeapLimitReached`) au script imbriqué
  et au script de plus haut niveau : le drapeau n'est consommé qu'à la profondeur d'appel nulle
  (`ExceptionError`). `Isolate.Cleanup` l'efface, ainsi que celui de `Isolate.HeapLimitReached`.
- **Terminaison** (`botify_termination.h`, `tools/patches/0008-termination.patch`) : une demande de
  terminaison (`TerminateExecution`, limite de heap) vaut pour toute l'exécution de plus haut niveau
  en cours, comme avec V8 9.0. V8 15.4 l'efface à l'entrée de la plupart de ses appels
  (`PrepareForExecutionScope`) : un callback Go qui faisait un appel V8 de plus après un script
  imbriqué terminé l'annulait. Les demandes sont comptées (par isolate, slot de données 2), et le
  compte relevé à la fin de chaque exécution de plus haut niveau et par `Cleanup` ; un compte
  différent signifie une terminaison due à l'exécution en cours. Les points d'entrée appelés depuis
  un callback (`BotifyEntry`, dans les macros `LOCAL_*` et `ISOLATE_SCOPE`) la redemandent alors
  avant leur appel V8 (`Promise.Then`/`Catch`, qui paniquent sur erreur, la retiennent pendant le
  leur), et `FunctionTemplateCallback` la rend effective avant de rendre la main au JS
  (`BotifyTerminateNow`, qui exécute un script trivial). Les points d'entrée qui lancent une
  exécution (`RunScript`, `UnboundScript.Run`, `Function.Call`/`NewInstance`,
  `PerformMicrotaskCheckpoint` : `BotifyRunScope`) redemandent sur leur thread une demande arrivée
  depuis la fin de l'exécution précédente, et annulent sinon celle que V8 garderait pour ce thread :
  V8 garde la demande dans l'état d'un thread, qu'un `Locker` pris par un autre thread archive. Le
  même patch remplace les `ToChecked()` des conversions et de `Has`/`Delete`/`Resolve`/`Reject` par
  des valeurs par défaut, qui arrêtaient le processus quand le JS appelé levait ou était terminé.
  Sans terminaison due, un point d'entrée ne fait que comparer deux compteurs de l'isolate (environ
  6 instructions sous callgrind) : `CallGoFromJS1000`, `CallJSFromGo`, `RunScriptTrivial`,
  `ObjectSetGet` sans écart significatif (benchmarks entrelacés, 10 tours, C++ en `-O2` comme le
  bridge) ; `Isolate::InContext()` n'est appelé que lorsqu'une terminaison est due. Attention : en
  mode source, `CGO_CXXFLAGS=-nostdinc++` remplace les options par défaut de cgo (`-g -O2`), et le
  C++ est compilé sans optimisation : pour mesurer, ajouter `-O2 -g0`.
- **Patchs des fichiers de tommie** : les modifications des fichiers de tommie (chemin JS→Go,
  garde-fous, pompe de `Cleanup`, limite de heap, terminaison, docs) sont des patchs,
  `tools/patches/*.patch` (0001 à 0010), que `tools/sync_tommie.sh`
  applique dans l'ordre après l'import : il s'arrête si l'un d'eux ne s'applique plus, en laissant
  appliqués les précédents. Le régénérer alors contre cet état : `git add -A` (sans committer), refaire
  la modification à la main, puis `git diff -- <fichiers modifiés> > tools/patches/<patch>`, et
  relancer l'import.
- **Locale ICU** (`botify_icu.go`) : `SetDefaultLocale` fixe la locale par défaut de l'ICU de V8 pour
  tout le processus (voir `MIGRATION.md` §3bis). Le symbole porte le suffixe de version de l'ICU
  (`uloc_setDefault_78`) : une montée de V8 qui change d'ICU casse le lien, mettre alors le suffixe
  à jour.
- **Fichiers repris par Botify** : `README.md`, `CHANGELOG.md`, `.gitignore` et les docs Botify
  (`BOTIFY.md`, `MIGRATION.md`, `CGO-DEPENDENCIES.md`) sont dans `tools/botify-owned.txt` : l'import
  ne les touche pas. Les nouvelles entrées du `CHANGELOG.md` de tommie sont à reporter à la main si
  utile. `tools/sync_tommie.sh` supprime à chaque import les workflows de tommie (seuls les
  `botify-*.yml` tournent ici : ceux de tommie exécutaient du code tiers non épinglé avec des
  secrets), `.fossa.yml`, Android et les sous-modules `deps/v8`/`deps/depot_tools`.
- `bench/` : comparaison avec `v0.6.0-botify-baseline` (V8 9.0, checkout attendu dans
  `../v8go-baseline`). `tools/docker/` : environnement de dev (le dossier parent de ce dépôt est monté
  sur `/src`, et la commande s'exécute dans `/src/<nom du dossier du dépôt>`, exporté en `V8GO_DIR` :
  un worktree ou un second clone s'utilisent de même).

## Bridges et pins des modules `deps/*`

Un consommateur ne prend pas `deps/<os>_<arch>` dans le commit de v8go qu'il requiert, mais dans les
versions des modules `deps/*` que le `go.mod` de v8go épingle (MVS). botify-ci vérifie les deux :

- `bridge-fresh` (`tools/check_bridge.sh`) : chaque module `deps/<os>_<arch>` a son archive de bridge
  et un `bridge.sha256` égal à `tools/bridge_hash.sh` ; le même job vérifie l'absence du shim et
  l'isolation du runtime C++ (`tools/check_cxx_runtime_isolated.sh`) ;
- `pinned-deps-fresh` (`tools/check_pinned_deps.sh`) : les modules `deps/*` épinglés par `go.mod`
  portent un `bridge.sha256` égal à `tools/bridge_hash.sh`, leur arbre git `deps/<os>_<arch>` est
  celui de HEAD (ce qui couvre aussi `cgo.go`, les archives V8 et les en-têtes, hors empreinte), et
  `bench/go.mod` épingle les mêmes versions. Le commit épinglé doit être dans le clone : le job
  récupère tout l'historique (`fetch-depth: 0`), donc les branches et les tags.

`tools/bridge_hash.sh` empreinte les `.cc`/`.h`, les `.go` à `//export`, `cgo.go`, `deps/v8_hash`,
`.github/actions/setup-clang/action.yml` (version de clang), `tools/build_bridge.sh`,
`tools/rename_cxx_runtime.sh` et `tools/cxx-runtime-rename.map`. Après une modification de l'un
d'eux :

1. pousser sur une branche autre que master. `botify-bridge` se déclenche seul pour `*.cc`, `*.h`,
   `tools/patches/**`, `deps/v8_hash`, `tools/cxx-runtime-rename.map`, `tools/rename_cxx_runtime.sh`,
   `tools/build_bridge.sh`, `tools/bridge_hash.sh`, `.github/actions/setup-clang/action.yml` et son
   propre fichier ; pour `cgo.go` ou un `.go` à `//export` seul, `bridge-fresh` échoue : lancer le
   workflow à la main sur la branche. Si les bridges ne correspondent plus aux sources, il pousse sur
   cette branche un commit « Rebuild prebuilt v8go bridges ». Le bot ne committe **jamais** sur
   master, même lancé à la main sur master : tout passe par une branche et une PR. D'ici là,
   `bridge-fresh` et `pinned-deps-fresh` échouent : c'est voulu ;
2. `git pull`, puis `tools/docker/dev.sh 'tools/pin_deps.sh <commit de bridges>'` (sha complet ou
   abrégé, ou tag de version, voir l'en-tête du script) ; committer `go.mod`, `go.sum` et
   `bench/go.mod`, et pousser ;
3. `botify-ci` tourne sur les PR (`pull_request`) et sur les push sur master : ce push relance donc
   `botify-ci` sur la PR ouverte pour la branche (sans PR, le lancer à la main,
   `workflow_dispatch`) ; le commit du bot, poussé avec le `GITHUB_TOKEN`, n'en déclenche pas. Le
   commit de pin ne change pas le C++, donc `pinned-deps-fresh` passe.

Un pin est obligatoire après chaque reconstruction des bridges, avant de tagger ou de fusionner.

**Commits épinglés et squash.** Les PR sont fusionnées en *squash* (c'est le cas de la PR de la
montée V8 15.4, pour garder l'historique de travail hors de master) : les commits de la branche, dont
le commit de bridges épinglé par `go.mod`, ne sont alors plus accessibles depuis master, et un module
épinglé doit rester accessible (Go le résout par son commit ; `pinned-deps-fresh` le cherche dans le
clone). Après le squash, refaire donc le pin sur le commit de master, puis, pour une version, sur des
tags (procédure ci-dessous). Une PR qui ne touche ni au bridge ni à `deps/` n'est pas concernée.

**Protection de master.** master n'est **pas** protégée aujourd'hui. Il est recommandé (action
d'administrateur du dépôt) de la protéger : passage par PR, et checks requis, sous leurs noms
affichés (ceux de `botify-ci.yml`) :

- `Prebuilt bridges match sources` (`bridge-fresh`) ;
- `Pinned deps modules carry the current bridges` (`pinned-deps-fresh`) ;
- `Vendored consumer build (go mod vendor)` (`vendor`, `tools/check_vendor.sh`, qui remplace le
  workflow Vendor Check de tommie, retiré à l'import) ;
- `Tests on linux amd64`, `Tests on linux arm64`, `Tests on darwin amd64`, `Tests on darwin arm64`,
  `Tests on windows amd64` ;
- `Consumer build and tests on glibc 2.34 (at most GLIBC_2.34)` (`glibc-floor`) ;
- `Fully static binary with g++ C++ code on linux amd64` et
  `Fully static binary with g++ C++ code on linux arm64` (`static-cxx-probe`) ;
- éventuellement `LeakSanitizer (source mode)` et `Cleanup soak (consumer mode)`.

## Publier une version (ex. v0.10.0)

1. **PR verte, squash-merge** : `botify-ci` passe sur la PR, `pinned-deps-fresh` compris ; la
   fusionner en squash, en **remplaçant le message proposé** par le titre de la PR et un corps court
   et propre (ce que la PR change, sans l'historique de travail). Le message par défaut concatène
   les messages de tous les commits de la branche (réglage `squash_merge_commit_message` =
   `COMMIT_MESSAGES` du dépôt) : sur une longue branche, des dizaines de messages de travail
   arriveraient sur master, dépôt public. Il est recommandé (action d'administrateur du dépôt) de
   régler `squash_merge_commit_title` = `PR_TITLE` et `squash_merge_commit_message` = `PR_BODY`.
   On obtient le commit **S** sur master, dont les `deps/<os>_<arch>` sont les
   bridges à jour (S contient le contenu du commit de bridges). Sur S, `pinned-deps-fresh` peut
   échouer (le commit de bridges épinglé n'est plus sur aucune branche une fois la branche
   supprimée) : c'est l'étape suivante qui le corrige.
2. **Tagger les modules `deps/*` sur S**, un tag par module, poussés **un par un, explicitement** :

   ```sh
   git fetch origin && S=$(git rev-parse origin/master)
   for d in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64 windows_amd64; do
     git tag "deps/$d/v0.10.0" "$S"
     git push origin "refs/tags/deps/$d/v0.10.0"
   done
   ```

   Jamais `git push --tags` : un clone local peut contenir les tags v0.7.0 à v0.9.0 de rogchap/v8go,
   absents de ce dépôt, qui seraient publiés avec.
3. **Épingler les tags** : sur une branche partant de S,
   `tools/docker/dev.sh 'tools/pin_deps.sh v0.10.0'` (le script vérifie
   `deps/<os>_<arch>/v0.10.0` pour chaque module, et que son arbre est celui de HEAD), committer
   `go.mod`, `go.sum`, `bench/go.mod`, et, dans la même PR, dater l'entrée de la version dans
   `CHANGELOG.md` (`### [v0.10.0] - unreleased` devient `### [v0.10.0] - <date du tag>`) ; ouvrir
   la PR, attendre `botify-ci` (dont `pinned-deps-fresh`), fusionner : commit **P** sur master. Le
   pin ne touche pas au C++ : le bridge n'est pas reconstruit.
4. **Tagger le module racine sur P** : `git tag v0.10.0 <P> && git push origin refs/tags/v0.10.0`.

Les consommateurs requièrent `github.com/botify-labs/v8go v0.10.0`, qui épingle les modules `deps/*`
à leurs tags : tout reste accessible indépendamment des branches.

## Mettre à jour V8

1. lancer `tools/docker/dev.sh 'tools/sync_tommie.sh <sha>'` (ou le workflow
   `botify-sync-upstream`, lancé chaque lundi : il ne fait rien si le commit de tommie est déjà
   importé, ni si sa branche `sync/tommie-<sha>` existe déjà, PR ouverte, pour ne pas écraser les
   commits de bridges et de pin poussés dessus ; pour refaire un import, supprimer d'abord la
   branche) sur une branche ; il retire le shim, renomme le runtime C++ des archives
   Linux (table régénérée : committer son diff avec les archives), applique les patchs et finit par
   `tools/check_cxx_runtime_isolated.sh`. Il remet dans `go.mod` les pins Botify des modules
   `deps/*` d'avant l'import (ceux de tommie désignent des commits de tommie, inexistants ici), donc
   les bridges précédents, jusqu'au pin de l'étape 2 ; `go.sum` garde les lignes Botify de ces pins,
   plus celles de tommie pour les modules tiers (`pin_deps.sh` le range avec `go mod tidy`). Il
   supprime aussi `.github/FUNDING.yml` et `.github/actions/checkout-depot-tools` de tommie ;
2. pousser : `botify-bridge` reconstruit les bridges (sinon le lancer à la main), puis épingler
   (`tools/pin_deps.sh`, ci-dessus) le commit de bridges, ou le commit d'import lui-même si aucune
   entrée du bridge n'a changé (pas de commit de bridges) ;
3. vérifier que `botify-ci` passe, `pinned-deps-fresh` compris, puis publier (ci-dessus).

## Benchmarks

- `tools/docker/dev.sh bench/run.sh` : baseline (V8 9.0, `../v8go-baseline`) contre la nouvelle
  version compilée comme chez un consommateur. Sections : `v8go-il` (benchmarks v8go entrelacés :
  `COUNT` tours d'un passage chacun, `-test.count 1`, l'ordre des deux versions alternant à chaque
  tour ; c'est ainsi qu'ont été produits les `v8go-il-*.txt` des résultats, dont le −22 % de geomean
  de `bench/results/2026-10-07-final/summary.md`), `v8go` (`-count` d'affilée par version, sensible
  aux dérives de la machine) et `soak`. Sans argument : `v8go-il` et `soak`.
- `botify-bench` (workflow manuel) : baseline contre nouvelle version sur macOS (amd64 et arm64),
  nouvelle version seule sur linux/arm64 et Windows (pas de baseline V8 9.0 pour ces plateformes).

Migration des consommateurs : `MIGRATION.md`. Dépendances cgo des consommateurs et nouvelles libs :
`CGO-DEPENDENCIES.md`.

## Piège clang-format

`clang-format` réécrit la première ligne `//go:build v8go_source` des fichiers `.cc` en
`// go:build v8go_source`. La contrainte de build est alors ignorée **sans erreur** : les `.cc` sont
compilés dans tous les builds, y compris ceux des consommateurs (sans clang ≥ 21 ni `-nostdinc++`,
qui échouent alors). `tools/sync_tommie.sh` place donc la contrainte entre des marqueurs
`// clang-format off` / `// clang-format on`. Après tout formatage, vérifier les lignes 1 à 3 de
chaque `.cc` (`head -3 *.cc`) : la ligne 2 doit être exactement `//go:build v8go_source`.
