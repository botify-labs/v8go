# Migrer de rogchap.com/v8go (fork Botify 6f9829d) vers github.com/botify-labs/v8go

## 1. Module

```diff
-replace rogchap.com/v8go => github.com/botify-labs/v8go v0.0.0-20211129082619-6f9829d18985
-require rogchap.com/v8go v0.6.1-0.20211110211436-d8d94c25bd2b
+require github.com/botify-labs/v8go v0.10.0
```

(`v0.10.0` une fois publiée, voir `CHANGELOG.md` ; d'ici là, une pseudo-version de la branche.)

```diff
-import "rogchap.com/v8go"
+import v8go "github.com/botify-labs/v8go"
```

Le code existant compile sans autre changement dans la plupart des cas (une seule incompatibilité de
compilation, §4). Les comportements qui changent sont au §3bis.

## 2. Toolchain

- **Linux / macOS : aucun changement.** V8 et le bridge C++ de v8go sont livrés précompilés et liés
  en statique ; aucun C++ n'est compilé chez le consommateur (gcc ou clang système, sans flags).
- **Plateformes** : linux amd64/arm64, darwin amd64/arm64, windows amd64. Sous linux/arm64, les
  libs cgo précompilées du consommateur doivent aussi exister pour arm64 (voir
  `CGO-DEPENDENCIES.md` §2.5).
- **Linux : glibc.** Les archives V8 de tommie sont construites contre le sysroot Debian bullseye
  de Chromium : V8 demande **glibc ≥ 2.31** (plancher annoncé par tommie, non testé par la CI de
  ce fork). Le job `glibc-floor` de botify-ci construit un binaire consommateur sur une
  distribution à glibc 2.34 (Rocky Linux 9, linux/amd64), vérifie qu'il ne demande pas plus que
  `GLIBC_2.34` et y lance les tests. Les glibc antérieures à 2.31 ne sont plus supportées.
- **Linux : le plancher effectif dépend de l'hôte de build.** Les archives V8 référencent des
  symboles de la libc/libm sans version (`fmod`, etc.) : l'éditeur de liens les lie aux versions par
  défaut de la glibc de l'hôte qui lie le binaire. Lié sur une glibc ≥ 2.38 (ex. ubuntu-24.04), le
  binaire exige `fmod@GLIBC_2.38` et ne démarre pas sur une glibc plus ancienne (2.34 par exemple).
  Lier sur la plus ancienne cible d'exécution (par exemple dans une image de la distribution cible,
  comme le job `glibc-floor`), ou vérifier le binaire :
  `objdump -T <binaire> | grep -oE 'GLIBC_[0-9.]+' | sort -Vu | tail -1` doit afficher au plus la
  version de la cible (`GLIBC_2.34` pour une glibc 2.34). Un lien entièrement statique n'est pas
  concerné.
- **Linux, lien totalement statique** (`-extldflags '-static'`) : fonctionne, y compris avec des libs
  C++ construites avec g++/libstdc++ (précompilées ou compilées depuis leurs sources). Le runtime C++
  de V8 (libc++ et libc++abi de Chromium) est renommé dans les archives Linux (suffixe `.v8cr`) : il
  ne définit plus aucun des symboles de libstdc++/libsupc++ (`__cxa_*`, `std::exception`,
  `operator new`…), chaque runtime garde ses exceptions et son RTTI, sans coût à l'exécution. Une lib
  C++ n'a donc rien à faire, ni prélink ni masquage de symboles. Voir `CGO-DEPENDENCIES.md` §2.2.
- **Linux, lien dynamique avec du C++ g++** : le même renommage corrige un défaut des versions
  précédentes de cette branche : le runtime de V8, lié dans l'exécutable, supplantait celui de
  `libstdc++.so`, et une exception levée dans libstdc++ (`std::stoi`…) finissait en `std::terminate`.
- **Windows amd64** : tout est statique en ABI MSVC (CRT statique `/MT`). **MinGW n'est pas
  supporté.**
  - Go ≥ 1.27, LLVM ≥ 21 (clang ciblant MSVC, par exemple `choco install llvm`), MSVC Build Tools +
    Windows SDK ;
  - `CC="clang -fuse-ld=lld"` et `CXX="clang++ -fuse-ld=lld"`. `-fuse-ld=lld` doit être dans `CC`
    (ou passé par `-ldflags=-extldflags=-fuse-ld=lld`) : c'est ainsi que Go détecte LLD, sinon il
    passe des flags que seul GNU ld accepte ;
  - clang doit être dans le `PATH` : Go découpe `CC` sur les espaces, un chemin complet
    (`C:\Program Files\LLVM\bin\clang.exe`) ne fonctionne pas ;
  - les packages cgo compilés depuis leurs sources sont recompilés par clang ciblant MSVC. La plupart
    du C passe tel quel, mais le code propre à GCC/MinGW peut échouer (`pthread.h`, `unistd.h`,
    extensions `__attribute__`, en-têtes réservés à MinGW) : à vérifier pour chaque dépendance ;
  - les libs précompilées en MinGW (`.a`) doivent être recompilées en MSVC `/MT` (`.lib`). Avec clang
    ciblant MSVC, `-lfoo` résout `foo.lib` : la `.lib` peut être placée à côté de la `.a` MinGW sans
    changer les `LDFLAGS`. Voir `CGO-DEPENDENCIES.md` §3.
- **Autres dépendances cgo** (pourquoi certaines libs précompilées ont dû être recompilées, et
  comment traiter une nouvelle lib) : voir `CGO-DEPENDENCIES.md`.
- **Développer v8go lui-même** : `-tags v8go_source`, clang ≥ 21, `CGO_CXXFLAGS=-nostdinc++`.
  Voir `BOTIFY.md`.

## 2bis. Mémoire et espace d'adressage

- **RSS plus élevé.** V8 15 consomme plus de mémoire que V8 9.0
  (`bench/results/2026-10-06-noshim/summary.md`, linux/amd64) : environ **+32 Mio de plateau par
  isolate long-lived** (soak v8go, un isolate et un contexte réutilisés : ~72 → ~104 Mio). C'est un
  coût fixe de V8 15 (tas, cage du sandbox et de la pointer compression, code), sans fuite.

  Relever les limites mémoire en conséquence. Le retrait de l'allocator shim ne change pas ces
  chiffres.
- **Grande réservation d'espace d'adressage virtuel.** V8 est compilé avec ses défauts x64/arm64
  (`deps/build.py`) : pointer compression et sandbox V8. Le premier isolate réserve la cage du sandbox
  (1 Tio d'adresses virtuelles, réservées mais pas allouées) et la cage du tas C++ d'Oilpan. Mesuré
  dans Docker (linux/amd64) : VmSize ≈ 2 Gio au démarrage, ≈ 1,4 Tio après le premier isolate, pour
  un RSS d'environ 17 Mio. Conséquences :
  - **`ulimit -v` / `RLIMIT_AS`** : sous une telle limite, V8 réserve moins, mais en dessous
    d'environ 128 Gio (96 Gio testés) le processus s'arrête à la création du premier isolate
    (`Fatal process out of memory: Oilpan: CagedHeap reservation.`, ou, vers 8 Gio,
    `Failed to reserve the virtual address space for the V8 sandbox`). Ne pas appliquer de limite
    d'espace d'adressage aux processus qui lient V8 ; limiter la mémoire par le RSS (cgroup, limite
    mémoire du conteneur) ;
  - les alertes et métriques fondées sur VSZ/VmSize ne sont plus significatives : utiliser le RSS.

## 3. Comportements conservés

- `Isolate.Cleanup()` et `Context.Cleanup()` gardent la même API. Les valeurs Go encore
  référencées par JS sont désormais conservées jusqu'à leur collecte par V8.
- `Isolate.Cleanup()` n'exécute jamais de JS ni de callback Go (§3bis) : comme avec l'ancien fork, les
  callbacks `FinalizationRegistry` ne s'exécutent jamais.
- `FunctionCallback` et `NewFunctionTemplate` sont inchangés : les callbacks existants compilent
  tels quels. Aucun test de la baseline n'a été supprimé.
- Le `malloc` du processus reste celui de la glibc (ou du système : zone par défaut sous macOS, UCRT
  sous Windows) pour tout le code C/C++ du consommateur, comme avec la baseline. Les libs V8
  précompilées de tommie embarquent l'*allocator shim* de PartitionAlloc, qui remplacerait
  `malloc`/`free`/`new`/`delete` pour tout le binaire : il est retiré à l'import
  (`tools/sync_tommie.sh`), et `tools/check_no_allocator_shim.sh` le vérifie.

## 3bis. Comportements modifiés

- **`Isolate.Cleanup()` exécute les tâches que V8 a postées pour l'isolate** (GC : memory reducer,
  etc. ; tâches qui règlent des promesses : compilation wasm asynchrone, `Atomics.waitAsync`), que
  v8go n'exécute nulle part ailleurs. Sans cela, un isolate long-lived accumulait ces tâches en
  mémoire native et ne lançait un GC majeur qu'à sa limite initiale. L'appeler régulièrement sur un
  isolate long-lived. Pendant `Cleanup()` :
  - aucun JS ni callback Go ne s'exécute : le JS qu'une tâche lancerait (callbacks
    `FinalizationRegistry`, réactions de promesses) est interrompu avant sa première instruction ; un
    callback de fonction renvoie `undefined` sans appeler Go, et ni le `PromiseRejectedCallback` ni
    le handler des messages console d'un `Inspector` ne sont appelés ;
  - les réactions de promesses encore en attente sont abandonnées, au lieu de s'exécuter à la fin du
    script suivant : celles des promesses réglées par ces
    tâches, et celles qu'un script terminé (`TerminateExecution`, limite de heap) a laissées, V8
    sautant le checkpoint de fin de script quand l'exécution est terminée. Les réactions des
    promesses que Go règle ou chaîne entre deux scripts (`PromiseResolver.Resolve`, `Promise.Then`)
    se sont déjà exécutées au retour de l'appel (politique de microtâches `kAuto`) ;
  - chaque `Cleanup()` élimine au plus 10 000 cellules mortes de `FinalizationRegistry` (une par
    tâche pompée, environ 8 ms) : des exécutions qui libèrent plus d'objets enregistrés que cela entre
    deux `Cleanup()` accumulent un arriéré (chaque cellule garde sa valeur associée), que les
    `Cleanup()` suivants résorbent ;
  - un `TerminateExecution` demandé pendant `Cleanup()` est annulé : le script suivant s'exécute
    normalement. Y compris celui qu'un watchdog, sur une autre goroutine, demande pour un script qui
    attend le verrou de l'isolate pendant que `Cleanup()` le tient : ce script s'exécute alors
    jusqu'au bout. Avec un tel watchdog, ne pas appeler `Cleanup()` en concurrence avec les
    scripts : l'appeler sur la goroutine qui les exécute, entre deux scripts.
- **`Isolate.Cleanup()` demande l'usage exclusif de l'isolate** : il remplace `Undefined(iso)` et
  `Null(iso)` sans synchronisation et libère les valeurs de l'isolate. Une autre goroutine qui
  garderait l'ancien `Undefined`/`Null` (ou une autre valeur libérée) utiliserait une valeur
  libérée. Ne pas exécuter de script ni utiliser de valeur de l'isolate sur d'autres goroutines
  pendant l'appel.
- **Les deux `Cleanup()` ne font rien s'ils sont appelés avec du JS de l'isolate sur la pile du
  thread appelant** (depuis un `FunctionCallback`) : ils libéreraient les valeurs des appels en
  cours. Appelés depuis une autre goroutine pendant qu'un script s'exécute, ils attendent le verrou
  (`Locker`) de l'isolate, donc la fin du script, puis s'exécutent : un callback Go qui attend une
  goroutine appelant `Cleanup()` provoque un interblocage. Les appeler une fois le script terminé.
- `Undefined(iso)` et `Null(iso)` obtenues avant `Isolate.Cleanup()` ne doivent plus être utilisées
  après : `Cleanup` libère les valeurs de l'isolate et les recrée, les re-récupérer.
- **Fonction d'un `Context` fermé** : une fonction Go d'un contexte fermé (`Close`), encore
  référencée par JS (passée à un autre contexte, par exemple), lève `Error: v8go: context closed`
  quand elle est appelée. L'`Error` vient du realm du contexte fermé : `e instanceof Error` est faux
  dans le contexte appelant ; tester `e.message`.
- **Terminaison et callbacks imbriqués** : un callback Go qui renvoie l'erreur d'un script imbriqué
  terminé (`TerminateExecution`, limite de heap) laisse la terminaison se propager jusqu'au script
  de plus haut niveau, au lieu de la relancer comme une exception que le JS pourrait attraper.
  Comme avec V8 9.0, une terminaison vaut pour toute l'exécution en cours (le script ou l'appel de
  fonction de plus haut niveau, ses callbacks Go et les scripts qu'ils lancent) : V8 15.4 l'efface
  à l'entrée de la plupart de ses appels, et un callback qui faisait un appel V8 de plus après un
  script imbriqué terminé (ou après un `toString` JS appelé par `Value.String()` et terminé)
  l'annulait ; v8go la redemande alors, le JS que lancent ces appels est interrompu avant sa
  première instruction, et le JS appelant est terminé dès le retour du callback. Les conversions
  `Value.Int32`/`Integer`/`Number`/`Uint32`, `Object.Has`/`Delete` et
  `PromiseResolver.Resolve`/`Reject` dont le JS (`valueOf`, getter, trap de proxy) lève une
  exception ou est terminé renvoient `0`/`false` au lieu d'arrêter le processus.
- **`TerminateExecution` entre deux scripts et threads** : V8 garde une demande de terminaison dans
  l'état d'un thread. Demandée pendant qu'aucun thread ne tient le verrou de l'isolate (un watchdog
  qui se déclenche entre deux appels cgo), elle était perdue si le script suivant tournait sur un
  autre thread OS (les goroutines changent de thread), et terminait à la place un script ultérieur
  du premier thread. v8go la reporte désormais sur l'exécution suivante (`RunScript`,
  `UnboundScript.Run`, `Function.Call`, `Function.NewInstance`, `PerformMicrotaskCheckpoint`),
  quel que soit son thread, et annule les demandes périmées. Les autres points d'entrée qui
  exécutent du JS hors d'une telle exécution (`Value.String()` sur un objet, `Object.Get` sur un
  getter, `JSONStringify`…) ne voient que la demande gardée pour leur thread. **Un watchdog doit donc
  redemander `TerminateExecution` périodiquement jusqu'au retour de l'exécution**, plutôt qu'une
  seule fois. `TerminateExecution` et `HeapLimitReached` ne doivent pas être appelés en concurrence
  avec `Isolate.Dispose()`.
- **Limite de heap** : elle n'arrête plus le processus (§4). Quand la limite approche, le callback de
  v8go relève la limite pour que V8 puisse interrompre le script proprement ; le script reçoit une
  erreur `ExecutionTerminated: heap limit reached` (`errors.Is(err, v8go.ErrHeapLimitReached)`), et la
  limite initiale est restaurée. Si c'est un script imbriqué (lancé par un callback Go) qui atteint
  la limite, son erreur et celle du script de plus haut niveau la reconnaissent toutes deux.
  L'isolate reste utilisable, mais l'état JS laissé par le script interrompu est indéterminé :
  recycler l'isolate (le disposer et en créer un autre) est recommandé. La limite peut aussi être
  atteinte sans qu'aucune erreur ne le signale (dans une réaction de promesse exécutée après le
  résultat du script, pendant une compilation, pendant un GC) : `iso.HeapLimitReached()` indique si
  elle l'a été depuis le dernier `Isolate.Cleanup()` (qui la remet à zéro), par exemple pour décider
  du recyclage avant `Cleanup()`. Une limite atteinte par un GC de la pompe de `Cleanup()` lui-même
  reste visible par `HeapLimitReached()` après l'appel.
- **Callbacks de `Promise.Then`/`Catch`** (inchangé, mais à connaître sur un isolate long-lived) :
  comme ceux des `FunctionTemplate`, les callbacks Go passés à `Then`/`Catch` sont gardés par
  l'isolate jusqu'à `Isolate.Dispose()`, même une fois la promesse réglée ou collectée.
- **Intl/ICU activé.** L'ancien fork était compilé sans i18n ; V8 embarque désormais son ICU (ICU 78,
  symboles suffixés `_78`, qui cohabitent avec une autre ICU liée par le consommateur). `Intl`,
  `toLocaleString`, `localeCompare`, `toLocaleDateString`, le nom du fuseau dans
  `Date.prototype.toString`… dépendent alors de l'hôte :
  - la locale par défaut vient de l'environnement (`LC_ALL`, `LC_MESSAGES` ou `LANG` sous Linux et
    macOS, réglages de l'utilisateur sous Windows) : le même script donne `1,234,567.891` sur un hôte
    et `1 234 567,891` sur un autre. Pour un résultat stable, appeler `v8go.SetDefaultLocale("en_US")`
    (par exemple) une fois, **avant de créer le premier isolate** ; elle vaut pour tout le processus ;
  - le fuseau horaire est lu une fois par processus : `TZ`, sinon `/etc/localtime` (Linux et macOS),
    réglages du système sous Windows. `SetDefaultLocale` ne le change pas.
- **Chemin JS→Go** (`bench/results/2026-10-07-callback/callback-perf.md`) :
  - fermer un `Context` (`Close`) **avant** de disposer son `Isolate` : un `Close` après `Dispose`,
    déjà invalide, prend maintenant le verrou d'un isolate libéré (comportement indéfini, en
    pratique plutôt un plantage qu'une corruption silencieuse) ;
  - de même, ne pas appeler `Context.Cleanup()` après `Isolate.Dispose()` : c'est invalide, et
    `Cleanup` prend lui aussi le verrou (`Locker`) de l'isolate libéré, ce qui plante désormais.
    Ordre correct : `ctx.Cleanup()` éventuel, `ctx.Close()`, puis `iso.Dispose()` ;
  - pas de `runtime.SetFinalizer` sur les valeurs d'un callback (`info.Args()[i]`, `info.This()`) :
    jusqu'à 4 arguments, ce sont des pointeurs intérieurs d'un bloc alloué par appel, et le runtime
    Go refuse le finaliseur ;
  - `Context.Close` prend le verrou de l'isolate : ne pas l'appeler depuis une goroutine qu'un
    callback en cours (JS en cours d'exécution sur cet isolate) attend, sous peine d'interblocage.

## 4. Changements d'API

Source : `docs/apidiff-6f9829d-to-tommie.txt` (`apidiff -incompatible`, une seule
incompatibilité de compilation) et comparaison des tests de la baseline avec les nouveaux (comportements).

| Avant (6f9829d) | Après | Adaptation |
|---|---|---|
| `NewIsolate` : `func() *Isolate` | `func(...IsolateOption) *Isolate` | Les appels `v8go.NewIsolate()` compilent sans changement. Seule l'utilisation de `NewIsolate` comme valeur de fonction casse : `var f func() *v8go.Isolate = v8go.NewIsolate` devient `f := func() *v8go.Isolate { return v8go.NewIsolate() }`. Options : `WithResourceConstraints(initial, max)` et `WithExceptionMessages()`. |
| `obj.Set("", v)` renvoie une erreur (clé vide refusée) | La clé vide est une propriété valide (`foo['']`). `Set("a", nil)` renvoie toujours une erreur (valeur nil invalide). | Avant : `err := obj.Set("", nil) // err != nil`. Après : `obj.Set("", "x")` réussit ; ne plus s'appuyer sur l'erreur pour rejeter une clé vide, la valider avant l'appel si besoin. |
| Messages d'erreur V8 9.0, ex. `SyntaxError: Unexpected identifier` | Messages de la version courante de V8, ex. `SyntaxError: Unexpected identifier 'js'` | Les consommateurs qui comparent des chaînes d'erreur (`err.Error()`, `JSError.Message`) doivent être mis à jour, de préférence en testant un préfixe ou `strings.Contains`. |
| Limite de heap atteinte : V8 9.0 arrête le processus (`Fatal javascript OOM in Reached heap limit`, puis `SIGILL`) | Le script est interrompu et `RunScript` renvoie une erreur de message `ExecutionTerminated: heap limit reached` (avant tommie v0.36 : le message générique `ExecutionTerminated: script execution has been terminated`, qui reste celui d'un `TerminateExecution`), reconnue par `errors.Is(err, v8go.ErrHeapLimitReached)`. L'isolate reste utilisable, avec sa limite initiale restaurée. | Tester `errors.Is(err, v8go.ErrHeapLimitReached)` plutôt que le texte. Ne plus compter sur un arrêt du processus (redémarrage par un superviseur) : traiter l'erreur, et disposer l'isolate si besoin. La limite se règle par `WithResourceConstraints`. |
| Les flags V8 (`SetFlags`) sont déjà globaux au processus, sans garantie documentée | Confirmé : `SetFlags` affecte aussi les scripts compilés en parallèle dans d'autres isolates | Appeler `v8go.SetFlags` une seule fois, au démarrage, avant de créer des isolates. Les tests ne doivent pas l'appeler en `t.Parallel()`. |
| `StartProfiling(title)` / `StopProfiling()` | Conservés. Ajout de `(*CPUProfiler).Do(title, fn)` qui encadre start/stop et arrête le profil si `fn` panique. `Do`, `StartProfiling` et `StopProfiling` paniquent sur un profiler ou un isolate disposé. | Optionnel : remplacer `p.StartProfiling("t"); run(); prof := p.StopProfiling("t")` par `prof := p.Do("t", run)`. |
| `NewFunctionTemplate(iso, func(*FunctionCallbackInfo) *Value)` seulement | Inchangé, plus `NewFunctionTemplateWithError(iso, func(*FunctionCallbackInfo) (*Value, error))` : l'erreur renvoyée est levée comme exception JS. | Optionnel : au lieu de `iso.ThrowException(...)` dans le callback, faire `return nil, err`. |

Ajouts sans impact sur les consommateurs existants (listés par `apidiff`) : `Isolate.LowMemoryNotification`,
`Isolate.WriteHeapSnapshot`, `Isolate.SetPromiseRejectedCallback`, `ErrHeapLimitReached`,
`Context.RetainedValueCount`, `Value.Release` / `FunctionCallbackInfo.Release`, méthodes `Symbol` sur
`Object` et `ObjectTemplate` (`SetSymbol`, `GetSymbol`, `HasSymbol`, `DeleteSymbol`), `Value.AsSymbol`,
`Value.AsException`, `Value.External`, `Value.StrictEquals`, `Value.TypeOf`, `NewError` et variantes,
`Inspector`, `Promise.ThenWithError` / `CatchWithError`, `JSError.ExceptionMessage` / `Unwrap`,
`FunctionTemplate.Inherit` / `InstanceTemplate` / `PrototypeTemplate`, etc. Ajouts propres à Botify : `SetDefaultLocale` et `Isolate.HeapLimitReached` (§3bis).
