# Migrer de rogchap.com/v8go (fork Botify 6f9829d) vers github.com/botify-labs/v8go

## 1. Module

```diff
-replace rogchap.com/v8go => github.com/botify-labs/v8go v0.0.0-20211129082619-6f9829d18985
-require rogchap.com/v8go v0.6.1-0.20211110211436-d8d94c25bd2b
+require github.com/botify-labs/v8go <version publiée>
```

```diff
-import "rogchap.com/v8go"
+import v8go "github.com/botify-labs/v8go"
```

Pour gojs, c'est le seul changement : l'import. Le reste du code n'a pas bougé.

## 2. Toolchain

- **Linux / macOS : aucun changement.** V8 et le bridge C++ de v8go sont livrés précompilés et liés
  en statique ; aucun C++ n'est compilé chez le consommateur (gcc ou clang système, sans flags).
- **Linux : glibc ≥ 2.34** pour les binaires qui lient le nouveau V8. Amazon Linux 2023 convient ;
  Amazon Linux 2 n'est plus supporté.
- **Linux : ce plancher dépend de l'hôte de build.** Les archives V8 référencent des symboles de la
  libc/libm sans version (`fmod`, etc.) : l'éditeur de liens les lie aux versions par défaut de la
  glibc de l'hôte qui lie le binaire. Lié sur une glibc ≥ 2.38 (ex. ubuntu-24.04), le binaire exige
  `fmod@GLIBC_2.38` et ne démarre pas sur Amazon Linux 2023 (glibc 2.34). Lier sur la plus ancienne
  cible d'exécution, c.-à-d. dans `amazonlinux:2023` (comme cbjsex, et le job `glibc-floor` de
  botify-ci), ou vérifier le binaire :
  `objdump -T <binaire> | grep -oE 'GLIBC_[0-9.]+' | sort -Vu | tail -1` doit afficher au plus
  `GLIBC_2.34`. Un lien entièrement statique n'est pas concerné.
- **Linux, lien totalement statique** (`-extldflags '-static'`) : fonctionne, y compris avec des libs
  C++ construites avec g++/libstdc++ (précompilées ou compilées depuis leurs sources). Le runtime C++
  de V8 (libc++ et libc++abi de Chromium) est renommé dans les archives Linux (suffixe `.v8cr`) : il
  ne définit plus aucun des symboles de libstdc++/libsupc++ (`__cxa_*`, `std::exception`,
  `operator new`…), chaque runtime garde ses exceptions et son RTTI, sans coût à l'exécution. Le
  prélink de liburlnorm (`lib/linux/liburlnorm_prelinked.a`, cdf) n'est donc plus nécessaire ; il
  reste sans danger. Une nouvelle lib C++ n'a rien à faire. Voir `CGO-DEPENDENCIES.md` §2.2.
- **Linux, lien dynamique avec du C++ g++** : le même renommage corrige un défaut des versions
  précédentes de cette branche : le runtime de V8, lié dans l'exécutable, supplantait celui de
  `libstdc++.so`, et une exception levée dans libstdc++ (`std::stoi`…) finissait en `std::terminate`.
- **Windows amd64** : tout est statique en ABI MSVC.
  - Go ≥ 1.27, LLVM ≥ 21 (`choco install llvm`), MSVC Build Tools + Windows SDK ;
  - `CC="clang -fuse-ld=lld"` et `CXX="clang++ -fuse-ld=lld"` ;
  - les packages cgo compilés depuis leurs sources n'ont rien à faire ; les libs précompilées en
    MinGW doivent être recompilées en MSVC `/MT` (voir `docs/superpowers/cgo-inventory.md`) :
    - liburlnorm : `lib/windows_msvc`, fourni à partir de la version publiée avec cette migration ;
    - igzip (cdf et pulse) : `igzip.lib` à côté de `libigzip.a` ;
    - zstd de gocdf : `zstd_windows.lib` à côté de `libzstd_windows.a` (via une PR sur gocdf).
  - Avec clang ciblant MSVC, `-lfoo` résout `foo.lib` : les `.lib` MSVC peuvent coexister avec les
    `.a` MinGW sans changer les LDFLAGS.
- **Autres dépendances cgo** (pourquoi liburlnorm, zstd et igzip ont dû être recompilées, et comment
  traiter une nouvelle lib) : voir `CGO-DEPENDENCIES.md`.
- **Développer v8go lui-même** : `-tags v8go_source`, clang ≥ 21, `CGO_CXXFLAGS=-nostdinc++`.
  Voir `BOTIFY.md`.

## 2bis. Mémoire et espace d'adressage

- **RSS plus élevé.** V8 15 consomme plus de mémoire que V8 9.0
  (`bench/results/2026-10-06-noshim/summary.md`, linux/amd64) :
  - environ **+13,5 Mio de RSS pic par worker gojs** (45,9 → 59,4 Mio, +29 %, soak gojs 100k) ;
  - environ **+32 Mio de plateau par isolate long-lived** (soak v8go, un isolate et un contexte
    réutilisés : ~72 → ~104 Mio).

  Relever les limites mémoire en conséquence (environ +15 Mio par worker gojs). Le retrait de
  l'allocator shim ne change pas ces chiffres.
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
    mémoire du pod) ;
  - les alertes et métriques fondées sur VSZ/VmSize ne sont plus significatives : utiliser le RSS.

## 3. Comportements conservés

- `Isolate.Cleanup()` et `Context.Cleanup()` gardent la même API. Les valeurs Go encore
  référencées par JS sont désormais conservées jusqu'à leur collecte par V8.
- Intl/ICU est disponible (ICU 78 interne à V8, indépendante de l'ICU 65 de liburlnorm).
- `FunctionCallback` et `NewFunctionTemplate` sont inchangés : les callbacks existants compilent
  tels quels. Aucun test de la baseline n'a été supprimé.

## 3bis. Comportements modifiés

- `Isolate.Cleanup()` exécute d'abord les tâches que V8 a postées pour l'isolate (GC : memory
  reducer, etc. ; callbacks `FinalizationRegistry`), que v8go n'exécute nulle part ailleurs. Sans
  cela, un isolate long-lived accumulait ces tâches en mémoire native et ne lançait un GC majeur
  qu'à sa limite initiale (fuite du soak, Task 12a). Conséquences :
  - du JS, et les callbacks Go qu'il appelle, peuvent s'exécuter pendant `Cleanup()` ;
  - ne pas appeler `Cleanup()` depuis un `FunctionCallback` ni pendant que du JS s'exécute ;
  - re-récupérer `Undefined(iso)` / `Null(iso)` après `Cleanup()` ;
  - un `TerminateExecution` (watchdog) ou le callback de limite de heap peut prendre effet pendant
    `Cleanup()` et affecter le `RunScript` suivant.
- V8 n'installe plus PartitionAlloc comme `malloc` du processus. Les libs V8 précompilées de
  tommie embarquent l'*allocator shim* de PartitionAlloc, qui remplace `malloc`/`free`/`new`/`delete`
  pour tout le binaire consommateur. Il est retiré à l'import (`tools/sync_tommie.sh`). Le `malloc`
  de la glibc (ou du système : zone par défaut sous macOS, UCRT sous Windows) reste donc en place
  pour tout le code C/C++ du consommateur (cgo, liburlnorm, zstd, igzip…), comme avec la baseline.
  `tools/check_no_allocator_shim.sh` le vérifie.
- Chemin JS→Go (Task 25, `bench/results/2026-10-07-callback/callback-perf.md`) :
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

Source : `docs/superpowers/apidiff-6f9829d-to-tommie.txt` (`apidiff -incompatible`, une seule
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
`FunctionTemplate.Inherit` / `InstanceTemplate` / `PrototypeTemplate`, etc.
