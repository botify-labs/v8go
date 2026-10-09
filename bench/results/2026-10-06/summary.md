# V8 9.0 (6f9829d) → V8 15.4.80.20 (deps/v8_hash b3d6849) — résultats du 2026-10-06

Machine : Intel Core Ultra 9 285H, 16 CPU visibles (Docker Desktop / WSL2, 31 GiB) — COUNT=10 — linux/amd64 (Docker, Go 1.27.1, gcc 12.2), nouvelle version compilée comme chez un consommateur (gcc + bridge précompilé). v8go : branche de pré-publication à la date de la campagne, baseline 6f9829d. `BenchmarkCleanup1000Values` tourne à part avec `-benchtime=2000x`, avec les mêmes réglages pour les deux versions. Détails : `env-194031.txt`.

## v8go
```
goos: linux
goarch: amd64
pkg: github.com/botify-labs/v8go/bench
cpu: Intel(R) Core(TM) Ultra 9 285H
                              │   baseline    │                   new                   │
                              │    sec/op     │     sec/op      vs base                 │
NewIsolate-16                    1.025m ±  2%    1.101m ±   3%    +7.41% (p=0.000 n=10)
NewContext-16                    211.2µ ±  2%    131.9µ ±   2%   -37.58% (p=0.000 n=10)
RunScriptTrivial-16              841.1n ±  1%    974.0n ±   2%   +15.80% (p=0.000 n=10)
CallJSFromGo-16                  927.0n ±  1%   1302.0n ±   2%   +40.46% (p=0.000 n=10)
CallGoFromJS1000-16              770.9µ ±  8%   1070.5µ ±   3%   +38.87% (p=0.000 n=10)
NewValueString-16                383.3n ±  3%    470.6n ±   2%   +22.76% (p=0.000 n=10)
ValueToGoString-16               1.564µ ±  1%    2.072µ ±   1%   +32.52% (p=0.000 n=10)
ObjectSetGet-16                  1.185µ ±  2%    1.432µ ±   2%   +20.84% (p=0.000 n=10)
JSONParse-16                    100.68µ ±  2%    27.92µ ±   1%   -72.27% (p=0.000 n=10)
JSONStringify-16                 60.51µ ±  1%    31.06µ ±   4%   -48.67% (p=0.000 n=10)
CompileLodashCold-16             2.906m ±  4%    3.474m ±   5%   +19.53% (p=0.000 n=10)
CompileLodashWithCodeCache-16    526.2µ ±  2%    638.6µ ±   6%   +21.36% (p=0.000 n=10)
StartupWithLodash-16             5.668m ±  1%    5.101m ±   5%    -9.99% (p=0.000 n=10)
JSRegex-16                       61.24µ ± 19%    38.85µ ±  19%   -36.56% (p=0.000 n=10)
JSObjects-16                     174.9µ ±  4%    268.8µ ± 210%   +53.74% (p=0.000 n=10)
JSArrays-16                      4.724m ±  2%    6.003m ±   8%   +27.08% (p=0.000 n=10)
JSStrings-16                     601.9µ ±  2%    523.0µ ±  17%   -13.11% (p=0.011 n=10)
JSJSON-16                       1531.4µ ±  2%    645.0µ ±   3%   -57.88% (p=0.000 n=10)
HeapPerIsolate-16                1.281m ±  2%    1.225m ±   4%    -4.32% (p=0.007 n=10)
HeapPerContext-16               11.921m ±  2%    7.477m ±   5%   -37.28% (p=0.000 n=10)
Cleanup1000Values-16             11.75µ ±  8%    44.48µ ±   2%  +278.54% (p=0.000 n=10)
geomean                          119.3µ          116.2µ           -2.64%

                              │    baseline    │                  new                   │
                              │      B/op      │     B/op      vs base                  │
NewIsolate-16                     144.0 ± 0%       176.0 ± 0%  +22.22% (p=0.000 n=10)
NewContext-16                     80.00 ± 0%       80.00 ± 0%        ~ (p=1.000 n=10) ¹
RunScriptTrivial-16               16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
CallJSFromGo-16                   68.00 ± 0%       68.00 ± 0%        ~ (p=1.000 n=10) ¹
CallGoFromJS1000-16             135.7Ki ± 0%     135.7Ki ± 0%        ~ (p=0.704 n=10)
NewValueString-16                 32.00 ± 0%       32.00 ± 0%        ~ (p=1.000 n=10) ¹
ValueToGoString-16                272.0 ± 0%       272.0 ± 0%        ~ (p=1.000 n=10) ¹
ObjectSetGet-16                   36.00 ± 0%       36.00 ± 0%        ~ (p=1.000 n=10) ¹
JSONParse-16                      16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSONStringify-16                16.00Ki ± 0%     16.00Ki ± 0%        ~ (p=1.000 n=10) ¹
CompileLodashCold-16            2.647Ki ± 4%     3.093Ki ± 6%  +16.84% (p=0.000 n=10)
CompileLodashWithCodeCache-16     16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
StartupWithLodash-16              272.0 ± 0%       304.0 ± 0%  +11.76% (p=0.000 n=10)
JSRegex-16                        16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSObjects-16                      16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSArrays-16                       16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSStrings-16                      16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSJSON-16                         16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
HeapPerIsolate-16                 240.0 ± 0%       272.0 ± 0%  +13.33% (p=0.000 n=10)
HeapPerContext-16               4.828Ki ± 0%     4.859Ki ± 0%   +0.65% (p=0.000 n=10)
Cleanup1000Values-16              0.000 ± 0%       0.000 ± 0%        ~ (p=1.000 n=10) ¹
geomean                                      ²                  +2.89%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                              │   baseline    │                  new                  │
                              │   allocs/op   │  allocs/op   vs base                  │
NewIsolate-16                    4.000 ± 0%      5.000 ± 0%  +25.00% (p=0.000 n=10)
NewContext-16                    5.000 ± 0%      5.000 ± 0%        ~ (p=1.000 n=10) ¹
RunScriptTrivial-16              1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
CallJSFromGo-16                  5.000 ± 0%      5.000 ± 0%        ~ (p=1.000 n=10) ¹
CallGoFromJS1000-16             7.746k ± 0%     7.746k ± 0%        ~ (p=1.000 n=10) ¹
NewValueString-16                2.000 ± 0%      2.000 ± 0%        ~ (p=1.000 n=10) ¹
ValueToGoString-16               2.000 ± 0%      2.000 ± 0%        ~ (p=1.000 n=10) ¹
ObjectSetGet-16                  2.000 ± 0%      2.000 ± 0%        ~ (p=1.000 n=10) ¹
JSONParse-16                     1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSONStringify-16                 1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
CompileLodashCold-16             1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
CompileLodashWithCodeCache-16    1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
StartupWithLodash-16             12.00 ± 0%      13.00 ± 0%   +8.33% (p=0.000 n=10)
JSRegex-16                       1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSObjects-16                     1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSArrays-16                      1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSStrings-16                     1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
JSJSON-16                        1.000 ± 0%      1.000 ± 0%        ~ (p=1.000 n=10) ¹
HeapPerIsolate-16                10.00 ± 0%      11.00 ± 0%  +10.00% (p=0.000 n=10)
HeapPerContext-16                304.0 ± 0%      305.0 ± 0%   +0.33% (p=0.000 n=10)
Cleanup1000Values-16             0.000 ± 0%      0.000 ± 0%        ~ (p=1.000 n=10) ¹
geomean                                     ²                 +1.93%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                              │   baseline   │                  new                  │
                              │     B/s      │     B/s       vs base                 │
JSONParse-16                    142.1Mi ± 2%   512.5Mi ± 1%  +260.68% (p=0.000 n=10)
JSONStringify-16                236.4Mi ± 1%   460.5Mi ± 4%   +94.80% (p=0.000 n=10)
CompileLodashCold-16            178.5Mi ± 4%   149.4Mi ± 5%   -16.33% (p=0.000 n=10)
CompileLodashWithCodeCache-16   986.1Mi ± 2%   812.6Mi ± 5%   -17.59% (p=0.000 n=10)
geomean                         277.3Mi        411.4Mi        +48.36%

                  │  baseline   │                 new                 │
                  │  heapB/iso  │  heapB/iso   vs base                │
HeapPerIsolate-16   744.6k ± 0%   191.4k ± 0%  -74.29% (p=0.000 n=10)

                  │  baseline   │                 new                 │
                  │  heapB/ctx  │  heapB/ctx   vs base                │
HeapPerContext-16   69.83k ± 0%   88.31k ± 0%  +26.46% (p=0.000 n=10)
```

## Endurance (100 000 itérations)
| Test | Version | RSS début → fin | Croissance 2e moitié | Verdict |
|---|---|---|---|---|
| v8go TestSoakCleanup | baseline | 59 → 72 MiB (plateau ~71 MiB dès 50k) | -0.57 % | PASS |
| v8go TestSoakCleanup | new | 55 → 104 MiB (plateau ~101-105 MiB dès 25k) | 1.13 % | PASS |

La nouvelle version passe le soak.

## Mémoire
| Mesure | baseline | new | Écart |
|---|---|---|---|
| RSS plateau du soak v8go (1 isolate + 1 contexte réutilisés) | ~71 MiB | ~103 MiB | +32 MiB |
| heapB/iso (tas V8 utilisé, isolate neuf + 1 contexte) | 744.6k | 191.4k | -74 % |
| heapB/ctx (tas V8 ajouté par contexte supplémentaire) | 69.83k | 88.31k | +26 % |

heapB/* est lu sans GC préalable : ces valeurs sont indicatives. La forte baisse de heapB/iso tient probablement à la comptabilité de V8 15 (espace read-only partagé hors de UsedHeapSize, non vérifié) et ne se retrouve pas dans le RSS. Le surcoût RSS (+32 MiB sur le soak v8go) ne vient pas de l'allocateur : sans le shim malloc de PartitionAlloc (voir plus bas), le soak v8go plafonne à 104 MiB. Il vient de V8 15 lui-même (tas, cage sandbox/pointer compression, code).

## Régressions > 5 % (p < 0.05)

### Cause principale : PartitionAlloc remplace malloc dans tout le process
Les bibliothèques V8 précompilées embarquent le shim d'allocation de Chromium (`deps/linux_amd64/libv8-3.a`, membre `871.allocator_shim.o`). Le binaire final exporte donc `malloc`/`free`/`calloc`/`realloc`/`operator new`/`delete` (`nm <binaire de test> | grep " T malloc"`). Toutes les allocations du process passent par PartitionAlloc : celles de V8, mais aussi celles du bridge (`new m_value`, `std::vector`, `unordered_map`), les `C.CString`/`C.free` de Go et les autres bibliothèques C liées au process.

Quand une slot span se vide, PartitionAlloc la décommitte (`madvise(MADV_DONTNEED)`), et les allocations suivantes refautent les pages. Sur `CallJSFromGo` à 2 M itérations :
- défauts de page mineurs : 7 000 en baseline, 102 000 en new, 12 000 en new sans shim ;
- temps système : 0.05 s, 0.29 s, 0.05 s ;
- 24 000 `madvise`, contre 237 en baseline. Selon `strace -k`, 2 374 des 2 635 `madvise` tracés viennent de `ContextCleanup` → `PartitionRoot::FreeInUnknownRoot` → `SlotSpanMetadata::DecommitIfPossible` → `DecommitSystemPages`.

Preuve A/B : même `libv8-3.a`, mais sans `allocator_shim.o` (copie de travail, non commitée), donc avec le malloc de glibc :

| Benchmark | baseline | new (commité) | new sans shim |
|---|---|---|---|
| RunScriptTrivial | 871 ns | 957 ns (+9.9 %) | 772 ns (-11.4 %) |
| CallJSFromGo | 924 ns | 1275 ns (+38.0 %) | 953 ns (~, p=0.086) |
| NewValueString | 375 ns | 464 ns (+23.8 %) | 356 ns (-5.0 %) |
| ValueToGoString | 1.56 µs | 2.06 µs (+31.8 %) | 1.58 µs (+1.1 %) |
| ObjectSetGet | 1.21 µs | 1.47 µs (+21.9 %) | 1.04 µs (-14.1 %) |
| CallGoFromJS1000 | 856 µs | 1053 µs (+23.0 %) | 1013 µs (+18.3 %) |
| NewIsolate | 1.03 ms | 1.09 ms (+5.7 %) | 1.03 ms (~) |
| Cleanup1000Values | 11.3 µs | 44.1 µs (+291 %) | 25.2 µs (+123 %) |

Hypothèses écartées, avec la preuve :
- **Hardening libc++ EXTENSIVE** : le bridge reconstruit avec `_LIBCPP_HARDENING_MODE_NONE` ne change rien. Geomean +1 %, non significatif, sauf CallGoFromJS1000 à +6 % *en défaveur* de NONE. La reconstruction EXTENSIVE est identique octet pour octet au `libv8go.a` commité.
- **Locker/Isolate::Scope/HandleScope/TryCatch par appel** : les macros `LOCAL_CONTEXT`/`ISOLATE_SCOPE` sont les mêmes dans les deux versions.
- **Plus d'instructions (V8 15, sandbox, unordered_map, Go)** : callgrind sur tout le process donne un nombre d'instructions par op à ±7 % entre new et baseline. CallJSFromGo : 14.9k contre 14.0k. NewValueString : 6.3k contre 5.9k. ObjectSetGet : 17.4k contre 18.5k. Les points d'entrée C++ en exécutent même moins en new. La régression ne vient donc pas de code en plus, mais de cycles perdus (défauts de page, cache), ce qui colle à l'allocateur. Le `unordered_map` de suivi des valeurs ne pèse que ~1.5 % des instructions de `FunctionCall`.
- **ctxMutex / migration de threads côté Go** : l'écart est le même avec GOMAXPROCS=1 et 16.

### Ligne par ligne
- **NewIsolate +7.4 %** : allocateur. Sans shim, identique à la baseline.
- **RunScriptTrivial +15.8 %, CallJSFromGo +40.5 %, NewValueString +22.8 %, ValueToGoString +32.5 %, ObjectSetGet +20.8 %** : allocateur PartitionAlloc. Les pages libérées par Cleanup sont décommittées puis refautent, et chaque malloc coûte plus cher. Sans shim, tout revient au niveau de la baseline ou en dessous.
- **CallGoFromJS1000 +38.9 %** : l'allocateur explique ~4 points. Le reste (+18 %) est propre au chemin callback JS→Go de V8 15 et du bridge : par argument, un `std::vector` sur le tas au lieu d'un VLA et un `unordered_map` de suivi au lieu d'un `vector`, plus un retour Go à deux valeurs (valeur, erreur). Pas investigué plus loin.
- **Cleanup1000Values +279 %** : d'une part l'allocateur (-43 % sans shim). D'autre part le coût voulu du correctif qui fait désormais pomper à `Isolate.Cleanup` les tâches de la plateforme V8, sans lequel la mémoire fuit. S'ajoute le parcours de l'`unordered_map`. Ce coût est hors timer dans les benchmarks par appel, mais payé une fois par appel de `Cleanup` entre deux exécutions.
- **CompileLodashCold +19.5 %, CompileLodashWithCodeCache +21.4 %** : parser/compilateur et désérialisation du code cache de V8 15. Dans l'A/B sans shim, WithCodeCache revient au niveau de la baseline (p=0.48). Pour Cold, l'A/B n'est pas concluant (passe très bruitée, ±121 %). Le code cache est bien accepté : 0.64 ms contre 3.5 ms à froid.
- **JSObjects +53.7 % (±210 %), JSArrays +27.1 %** : non confirmés. JSObjects est bimodal en new : 6 échantillons à ~200 µs, 4 entre 700 µs et 1 ms (GC majeur pendant l'échantillon). JSArrays dérive au fil de la passe (5.5 → 6.5 ms). Dans deux passes A/B ultérieures, JSArrays donne -7 % puis n.s., et JSObjects +13 % puis +24 % avec ±118-177 %. C'est un effet de la planification du GC de V8 15 sur un benchmark qui alloue beaucoup, pas une régression stable du JIT.
- **heapB/ctx +26 %** : coût mémoire d'un contexte V8 15 (voir Mémoire).
- **B/op Go** (+32 octets sur NewIsolate/HeapPerIsolate/StartupWithLodash, +17 % sur CompileLodashCold) : allocations Go marginales, sans impact.

### Proposition (non appliquée dans cette tâche)
Construire V8 sans le shim malloc de PartitionAlloc : ajouter les arguments gn `use_partition_alloc_as_malloc=false` et `use_allocator_shim=false` dans `deps/build.py`, puis relancer `bench/run.sh` et la suite de tests. Une bibliothèque embarquée ne devrait pas remplacer le malloc du process hôte, ni pour la performance, ni pour la cohabitation avec les autres bibliothèques C du process. Pistes secondaires : remplacer le `unordered_map` de suivi par un vecteur indexé avec réutilisation des slots, et éviter le `std::vector` alloué sur le tas à chaque callback.

Les preuves brutes (sorties benchstat des A/B, callgrind, strace, `/usr/bin/time -v`) figurent dans le rapport de travail de l'analyse du shim, non publié.
