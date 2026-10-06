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
- **Linux, lien totalement statique** (`-extldflags '-static'`) : toute lib C++ précompilée avec
  g++/libstdc++ liée avec V8 dans un binaire entièrement statique doit être « prélinkée » avec sa
  propre libstdc++ privée, sinon elle entre en conflit avec le libstdc++ de V8. liburlnorm (cdf) est
  déjà traitée : elle fournit `lib/linux/liburlnorm_prelinked.a` (voir `liburlnorm/scripts/prelink_linux.sh`
  dans cdf). Toute autre lib du même type doit suivre la même procédure. Les liens dynamiques ne sont
  pas concernés.
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
- **Développer v8go lui-même** : `-tags v8go_source`, clang ≥ 21, `CGO_CXXFLAGS=-nostdinc++`.
  Voir `BOTIFY.md`.

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

## 4. Changements d'API

Source : `docs/superpowers/apidiff-6f9829d-to-tommie.txt` (`apidiff -incompatible`, une seule
incompatibilité de compilation) et comparaison des tests de la baseline avec les nouveaux (comportements).

| Avant (6f9829d) | Après | Adaptation |
|---|---|---|
| `NewIsolate` : `func() *Isolate` | `func(...IsolateOption) *Isolate` | Les appels `v8go.NewIsolate()` compilent sans changement. Seule l'utilisation de `NewIsolate` comme valeur de fonction casse : `var f func() *v8go.Isolate = v8go.NewIsolate` devient `f := func() *v8go.Isolate { return v8go.NewIsolate() }`. Options : `WithResourceConstraints(initial, max)` et `WithExceptionMessages()`. |
| `obj.Set("", v)` renvoie une erreur (clé vide refusée) | La clé vide est une propriété valide (`foo['']`). `Set("a", nil)` renvoie toujours une erreur (valeur nil invalide). | Avant : `err := obj.Set("", nil) // err != nil`. Après : `obj.Set("", "x")` réussit ; ne plus s'appuyer sur l'erreur pour rejeter une clé vide, la valider avant l'appel si besoin. |
| Messages d'erreur V8 9.0, ex. `SyntaxError: Unexpected identifier` | Messages de la version courante de V8, ex. `SyntaxError: Unexpected identifier 'js'` | Les consommateurs qui comparent des chaînes d'erreur (`err.Error()`, `JSError.Message`) doivent être mis à jour, de préférence en testant un préfixe ou `strings.Contains`. |
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
