# Chemin JS→Go : coût par callback — 2026-10-07

Régression de départ (`../2026-10-06-noshim/summary.md`) : `BenchmarkCallGoFromJS1000` (1000 appels
JS→Go par op) passait de 744 µs (baseline 6f9829d, V8 9.0) à 899 µs (+20,9 %). Un script qui appelle
des fonctions Go exposées au JS peut en faire des centaines par exécution.

Machine : Intel Core Ultra 9 285H, Docker Desktop / WSL2, Go 1.27, gcc 12.2 (build consommateur :
bridge précompilé). Chaque variante est un binaire de test lié à son propre bridge ; les variantes
tournent **entrelacées** (10 tours d'un run chacune, `run_variants.sh`), pour répartir la dérive de la
machine. `Cleanup1000Values` tourne à `-benchtime=2000x`.

## Les patchs

Les fichiers de tommie sont modifiés par `tools/patches/*.patch`, appliqués dans l'ordre par
`tools/sync_tommie.sh` après chaque import.

| Variante | Contenu |
|---|---|
| base | v8go 6f9829d (V8 9.0, rogchap) |
| v0 | branche de pré-publication avant l'optimisation du chemin JS→Go |
| v1 | + `0001-value-tracker` : `botify_values.h`, valeurs d'un contexte dans un vecteur (id = index + 1, retrait par échange avec la dernière) au lieu de l'`unordered_map<long, m_value*>` de tommie ; `Cleanup` compacte en une passe et libère les gros lots par adresse de handle décroissante |
| v2 | + `0002-context-pointer` : `botify_context.h`, le `m_ctx` rangé dans les *embedder data* du contexte V8 (slot 2) ; le callback ne rappelle plus Go (`goContext`) pour le trouver |
| v3 | + `0003-callback-fast-path` : pas de `Locker`/`Isolate::Scope` dans `FunctionTemplateCallback` (V8 l'appelle isolate verrouillé et entré), pas de `Global` temporaire par valeur, arguments dans un tableau sur la pile (≤ 7) |
| v4 | + `0004-callback-frame` (Go) : `goFunctionCallback` alloue info, receveur et arguments en un bloc (≤ 2 ou ≤ 4 arguments) au lieu de 3 + n — **version finale** |
| v4sp | v4 avec le `release_if` de la première version (`std::stable_partition`) |
| nosort | (passe 1) v4 sans le tri par adresse des valeurs libérées |

## Résultats — version finale (`final/`, 10 tours entrelacés)

```
                           │    base     │      v0 (avant)      │     v1      │     v2      │     v3      │    v4 (final)     │    v4sp     │
                           │   sec/op    │   sec/op   vs base   │   sec/op    │   sec/op    │   sec/op    │  sec/op  vs base  │   sec/op    │
RunScriptTrivial-16           812.9n ± 6%   730.5n ± 4%  -10.14%   708.7n ± 4%   723.4n ± 2%   700.1n ± 4%   744.9n ± 5%  -8.36%  730.4n ± 4%
CallJSFromGo-16               895.9n ± 6%   962.8n ± 6%   +7.48%   802.1n ± 4%   797.7n ± 7%   793.2n ± 5%   810.2n ± 7%  -9.56%  809.6n ± 6%
CallGoFromJS1000-16          1107.3µ ±29%  1210.0µ ±25%   +9.28%   501.5µ ±11%   453.7µ ±14%   415.0µ ±16%   389.5µ ±16% -64.83%  391.4µ ± 5%
CallGoFromJS1000Cleanup-16    465.7µ ± 9%   654.6µ ± 8%  +40.58%   509.0µ ± 5%   470.3µ ±10%   420.8µ ± 8%   407.7µ ±12% -12.44%  411.4µ ± 8%
NewValueString-16             387.1n ± 7%   357.7n ± 6%   -7.58%   331.3n ± 7%   328.4n ± 8%   328.8n ± 7%   333.6n ± 8% -13.81%  337.5n ± 6%
ObjectSetGet-16              1127.5n ± 2%   974.2n ± 4%  -13.60%   914.6n ± 4%   905.7n ± 7%   913.8n ± 4%   914.8n ± 4% -18.87%  893.0n ± 4%
Cleanup1000Values-16          10.58µ ± 6%   23.39µ ± 2% +120.98%   14.58µ ± 5%   14.50µ ± 6%   14.54µ ± 2%   14.69µ ± 4% +38.85%  14.88µ ± 2%
geomean                       7.786µ        8.933µ       +14.74%   6.754µ        6.575µ        6.366µ        6.379µ      -18.07%  6.373µ
```

Tableau complet, avec les p-values et les colonnes vs base : `final/benchstat.txt`. Toutes les
valeurs de v4 sont significatives (p ≤ 0,005, n = 10). `CallGoFromJS1000` de la baseline est bruité
dans cette passe (± 29 %) ; la passe 1, plus calme, donne le même ordre de grandeur.

| allocs/op, B/op | base | v0..v3 | v4 |
|---|---|---|---|
| CallGoFromJS1000 | 7746, 135,7 Ki | 7746, 135,7 Ki | **2746 (-64,5 %), 127,9 Ki (-5,8 %)** |

## Contribution de chaque patch (passe 1, `pass1/`)

Passe 1 : mêmes conditions, machine plus calme, avant deux retouches (le bloc Go était unique, de
160 octets, au lieu de 112/160 ; `release_if` utilisait `stable_partition`).

| Benchmark | base | v0 | v1 (0001) | v2 (+0002) | v3 (+0003) | v4 (+0004) | nosort |
|---|---|---|---|---|---|---|---|
| CallGoFromJS1000 | 790,2 µs | 930,0 µs (+17,7 %) | 434,3 µs (-45,0 %) | 397,4 µs (-49,7 %) | 346,5 µs (-56,2 %) | 340,6 µs (-56,9 %) | 708,8 µs (-10,3 %) |
| CallGoFromJS1000Cleanup | 419,0 µs | 617,0 µs (+47,3 %) | 473,9 µs (+13,1 %) | 433,8 µs (+3,5 %) | 392,3 µs (-6,4 %) | 380,8 µs (-9,1 %) | 388,1 µs |
| CallJSFromGo | 855,7 ns | 906,1 ns (+5,9 %) | 767,0 ns | 757,5 ns | 765,9 ns | 757,6 ns (-11,5 %) | 800,1 ns |
| Cleanup1000Values | 10,24 µs | 23,83 µs (+132,7 %) | 14,34 µs (+40,1 %) | 14,54 µs | 14,42 µs | 14,53 µs (+41,9 %) | 14,53 µs |
| geomean (7 benchs) | | +18,4 % | -9,5 % | -11,9 % | -14,9 % | -15,5 % | -5,0 % |

- `CallGoFromJS1000` accumule 400 000 valeurs entre deux `Cleanup` (toutes les 100 ops) : il mesure
  surtout le suivi des valeurs. `CallGoFromJS1000Cleanup` (1000 appels puis `Cleanup`, chronométré :
  un isolate réutilisé entre deux exécutions) mesure le coût par appel.
- 0004 vs 0003, en direct (`benchstat v3 v4`) : -3,1 % (passe finale, p = 0,035) et -3,0 % (passe 1,
  p = 0,029) sur `CallGoFromJS1000Cleanup`, et 64 % d'allocations Go en moins.
- `release_if` en une passe vs `stable_partition` : non significatif en temps (`v4sp`), -11 % d'instructions
  par valeur dans `ContextCleanup` (callgrind, 182 vs 205), sans tampon temporaire : gardé.
- Le tri par adresse (`nosort`) : sans lui, `CallGoFromJS1000` est 2,1× plus lent (708,8 vs 340,6 µs).

## callgrind (`callgrind/`)

Instructions par callback, `(Ir à 40 ops − Ir à 20 ops) / 20 000`, `CallGoFromJS1000`, coût propre
(`GODEBUG=asyncpreemptoff=1`, sinon callgrind s'arrête sur une assertion à la réception de SIGURG) :

| catégorie | base | v0 | v1 | v2 | v3 | v4 |
|---|---|---|---|---|---|---|
| **total** | **8841** | **10568** | **8653** | **7981** | **7229** | **6857** |
| V8 | 2840 | 2156 | 2039 | 2127 | 1702 | 1717 |
| malloc/free (libc) | 2427 | 4048 | 2834 | 2755 | 2442 | 2056 |
| transitions cgo | 1696 | 1729 | 1729 | 1329 | 1329 | 1329 |
| C++ v8go | 577 | 1131 | 756 | 694 | 629 | 629 |

(détail par fonction : `callgrind/CallGoFromJS1000-per-callback.txt` ; le Go de la baseline,
`rogchap.com/v8go`, y est classé « other ».)

- **v0 → v1 (-1915)** : la table de hachage de tommie (`__hash_table` 296 + 103), l'allocation d'un nœud
  par valeur (`_int_malloc` 1030 → 515, `_int_free` 819 → 443), le `memset` des rehash (168).
- **v1 → v2 (-672)** : la transition C→Go de `goContext` (`cgocallback`, `exitsyscall`,
  `reentersyscall`… 1729 → 1329) et sa recherche dans le registre.
- **v2 → v3 (-752)** : le `Global` temporaire (deux `GlobalHandles::Create` et un `Release` par valeur :
  `Create` 414 → 260, `NodeSpace::Release` 266 → 152), le `Locker` (dont `Heap::SetStackStart` /
  `Stack::GetStackStart`, nouveaux dans V8 15) et l'`Isolate::Scope`.
- **v3 → v4 (-372)** : les allocations Go (`mallocgcSmallScanNoHeader*` 606 → 121).

`ContextCleanup` seul (`--toggle-collect=ContextCleanup`, par valeur libérée) : base 175, v0 388,
v4sp 205, v4 182 instructions. Avec `--cache-sim=yes` : base 186 Ir et 1,03 défaut D1 en lecture par
valeur, v4 206 Ir et 1,03 (`callgrind/ContextCleanup-cache-sim.txt`).

## Ce qui reste

- **Coût par callback : sous la baseline.** -22 % d'instructions, -12 % en temps sur
  `CallGoFromJS1000Cleanup`, -65 % sur `CallGoFromJS1000`.
- **`Cleanup1000Values` : +39 % (10,6 → 14,7 µs pour 1000 valeurs, +4 ns par valeur)**, contre +121 %
  avant. Les instructions par valeur sont au niveau de la baseline (182 vs 175 ; `free` de la libc et
  la libération du `Global` en font l'essentiel dans les deux versions) et le cache simulé ne montre
  pas plus de défauts en lecture (1,03 par valeur des deux côtés). La seule différence mesurée est dans
  la libération des handles `Global` par V8 15 (`GlobalHandles::Destroy` → `NodeSpace::Release`, qui
  n'est plus inliné) : 0,37 défaut D1 en écriture par valeur, contre 0,12 dans `Destroy` de V8 9.
  L'écart de temps est donc d'origine mémoire, dans V8 : le réduire demanderait de modifier V8, ou de
  ne plus créer un `Global` par valeur (changement d'API).
- Le reste des transitions cgo (1329 instructions par callback : un appel C→Go et les appels Go→C
  du callback lui-même) est inhérent à l'API.

## Validation

- tests source `-race` et consommateur `-race` (`./...`) ; LeakSanitizer (`-tags "leakcheck v8go_source"`) ;
  `tools/check_bridge.sh linux_amd64`, `tools/check_no_allocator_shim.sh`, en-têtes des `.cc` : OK.
- soak 100k (`../soak/v8go-new.csv`) : RSS 54 → 103 MiB, croissance sur la seconde moitié 1,58 %
  (limite 5 %), 20 s. Un premier run, lancé pendant un callgrind et un build (59 s au lieu de 20),
  avait échoué à 6,42 % : l'échantillon du milieu tombait dans un creux de la dent de scie du GC
  (98,4 MiB à 51k, 96,7 MiB à 41k), le plateau restant sous celui d'avant (≈ 105 vs ≈ 109 MiB).
- Mutations : sans la mise à jour de l'id de la valeur déplacée (`erase`), ou sans la renumérotation
  des valeurs gardées (`release_if`), les tests `botify_values_test.go` plantent (SIGSEGV).
- `tools/sync_tommie.sh 0ffc991…` relancé : les patchs s'appliquent et l'arbre est identique, à
  `go.mod`/`go.sum` près, que l'import remet à ceux de tommie et que `tools/pin_deps.sh` repinne
  (comportement existant).
