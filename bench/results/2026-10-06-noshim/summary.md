# V8 9.0 (6f9829d) → V8 15.4.80.20 (deps/v8_hash b3d6849), sans allocator shim — résultats du 2026-10-06

Machine : Intel Core Ultra 9 285H, 16 CPU visibles (Docker Desktop / WSL2, 31 GiB) — COUNT=10 — linux/amd64 (Docker, Go 1.27.1, gcc 12.2), nouvelle version compilée comme chez un consommateur (gcc + bridge précompilé). v8go 40178f3 (libs V8 sans l'*allocator shim* de PartitionAlloc, retiré par `tools/sync_tommie.sh`), baseline 6f9829d ; cdf upgrade e593b27, baseline 97ab17b (+ require goja temporaire, voir `bench/run.sh`). `BenchmarkCleanup1000Values` tourne à part avec `-benchtime=2000x`, mêmes réglages pour les deux versions. Détails : `env-213246.txt`.

La campagne précédente, **avec** le shim, est `../2026-10-06/` (Task 14). Cette campagne la remplace.

## Avec shim / sans shim

Les deux campagnes ont tourné sur la même machine, à quelques heures d'écart. La baseline elle-même a varié entre les deux passes : geomean -3.7 % sur v8go, -5.8 % sur gojs (`v8go-baseline-drift.txt`, `gojs-baseline-drift.txt`). La comparaison directe des deux « new » (`v8go-shim-vs-noshim.txt`, `gojs-shim-vs-noshim.txt`) inclut cette dérive. La comparaison fiable est donc l'écart new/baseline **mesuré dans chaque passe** :

| Benchmark | new/baseline avec shim (Task 14) | new/baseline sans shim (cette passe) | new sans shim / new avec shim (direct, dérive incluse) |
|---|---|---|---|
| NewIsolate | +7.4 % | **-5.7 %** | -14.2 % |
| RunScriptTrivial | +15.8 % | **-10.5 %** | -21.7 % |
| CallJSFromGo | +40.5 % | **+2.8 %** | -28.1 % |
| CallGoFromJS1000 | +38.9 % | **+20.9 %** | -16.0 % |
| NewValueString | +22.8 % | **-6.1 %** | -24.9 % |
| ValueToGoString | +32.5 % | **+2.6 %** | -23.8 % |
| ObjectSetGet | +20.8 % | **-18.0 %** | -31.5 % |
| CompileLodashCold | +19.5 % | **-8.1 %** | -26.7 % |
| CompileLodashWithCodeCache | +21.4 % | **-11.9 %** | -30.8 % |
| JSArrays | +27.1 % | **-6.6 %** | -26.7 % |
| Cleanup1000Values | +278.5 % | **+123.8 %** | -46.5 % |
| geomean v8go | -2.6 % | **-18.8 %** | -19.7 % |
| gojs V8_Cleanup/* | ~ à +14.8 % | **-4.6 % à +3.6 %** | -12.1 % à -17.4 % |
| gojs V8_ColdStart/* | -4.1 % à -9.2 % | **-12.5 % à -14.2 %** | -10.1 % à -13.6 % |
| gojs V8_CleanupOnly | +61.6 % | **+13.4 %** | -31.3 % |
| geomean gojs | +3.6 % | **-5.8 %** | -14.4 % |

Toutes les valeurs de la colonne « sans shim » sont significatives (p < 0.05, n=10), sauf gojs V8_Cleanup/QuerySelectors (n.s., p=0.075).

## v8go
```
goos: linux
goarch: amd64
pkg: github.com/botify-labs/v8go/bench
cpu: Intel(R) Core(TM) Ultra 9 285H
                              │   baseline    │                  new                   │
                              │    sec/op     │    sec/op      vs base                 │
NewIsolate-16                   1001.0µ ±  1%   944.2µ ±   2%    -5.67% (p=0.000 n=10)
NewContext-16                    208.2µ ±  1%   127.7µ ±   2%   -38.66% (p=0.000 n=10)
RunScriptTrivial-16              852.8n ±  1%   763.1n ±   2%   -10.52% (p=0.000 n=10)
CallJSFromGo-16                  910.7n ±  2%   936.3n ±   2%    +2.81% (p=0.000 n=10)
CallGoFromJS1000-16              744.2µ ±  2%   899.4µ ±   3%   +20.86% (p=0.000 n=10)
NewValueString-16                376.2n ±  1%   353.5n ±   2%    -6.05% (p=0.000 n=10)
ValueToGoString-16               1.538µ ±  1%   1.578µ ±   1%    +2.60% (p=0.000 n=10)
ObjectSetGet-16                 1196.5n ±  1%   980.8n ±   2%   -18.03% (p=0.000 n=10)
JSONParse-16                    100.91µ ±  2%   26.47µ ±   1%   -73.77% (p=0.000 n=10)
JSONStringify-16                 58.48µ ±  1%   28.80µ ±   1%   -50.75% (p=0.000 n=10)
CompileLodashCold-16             2.770m ±  2%   2.545m ±   1%    -8.12% (p=0.000 n=10)
CompileLodashWithCodeCache-16    501.8µ ±  1%   442.1µ ±   1%   -11.91% (p=0.000 n=10)
StartupWithLodash-16             5.440m ±  1%   4.228m ±   1%   -22.29% (p=0.000 n=10)
JSRegex-16                       48.87µ ± 44%   35.79µ ±  15%   -26.77% (p=0.000 n=10)
JSObjects-16                     166.1µ ±  5%   183.9µ ± 202%   +10.73% (p=0.000 n=10)
JSArrays-16                      4.712m ±  0%   4.400m ±   1%    -6.62% (p=0.000 n=10)
JSStrings-16                     572.7µ ±  1%   481.7µ ±   1%   -15.88% (p=0.000 n=10)
JSJSON-16                       1527.5µ ±  2%   618.7µ ±   3%   -59.50% (p=0.000 n=10)
HeapPerIsolate-16                1.246m ±  1%   1.083m ±   2%   -13.10% (p=0.000 n=10)
HeapPerContext-16               11.299m ±  1%   6.861m ±   2%   -39.28% (p=0.000 n=10)
Cleanup1000Values-16             10.63µ ±  1%   23.78µ ±   1%  +123.75% (p=0.000 n=10)
geomean                          114.9µ         93.33µ          -18.81%

                              │    baseline    │                  new                   │
                              │      B/op      │     B/op      vs base                  │
NewIsolate-16                     144.0 ± 0%       176.0 ± 0%  +22.22% (p=0.000 n=10)
NewContext-16                     80.00 ± 0%       80.00 ± 0%        ~ (p=1.000 n=10) ¹
RunScriptTrivial-16               16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
CallJSFromGo-16                   68.00 ± 0%       68.00 ± 0%        ~ (p=1.000 n=10) ¹
CallGoFromJS1000-16             135.7Ki ± 0%     135.7Ki ± 0%        ~ (p=0.825 n=10)
NewValueString-16                 32.00 ± 0%       32.00 ± 0%        ~ (p=1.000 n=10) ¹
ValueToGoString-16                272.0 ± 0%       272.0 ± 0%        ~ (p=1.000 n=10) ¹
ObjectSetGet-16                   36.00 ± 0%       36.00 ± 0%        ~ (p=1.000 n=10) ¹
JSONParse-16                      16.00 ± 0%       16.00 ± 0%        ~ (p=1.000 n=10) ¹
JSONStringify-16                16.00Ki ± 0%     16.00Ki ± 0%        ~ (p=1.000 n=10) ¹
CompileLodashCold-16            2.491Ki ± 7%     2.326Ki ± 2%   -6.62% (p=0.000 n=10)
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
geomean                                      ²                  +1.80%                ²
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
JSONParse-16                    141.8Mi ± 2%   540.5Mi ± 1%  +281.26% (p=0.000 n=10)
JSONStringify-16                244.6Mi ± 1%   496.7Mi ± 1%  +103.03% (p=0.000 n=10)
CompileLodashCold-16            187.3Mi ± 2%   203.9Mi ± 1%    +8.84% (p=0.000 n=10)
CompileLodashWithCodeCache-16   1.010Gi ± 1%   1.146Gi ± 1%   +13.52% (p=0.000 n=10)
geomean                         286.3Mi        503.5Mi        +75.86%

                  │  baseline   │                 new                 │
                  │  heapB/iso  │  heapB/iso   vs base                │
HeapPerIsolate-16   744.6k ± 0%   191.4k ± 0%  -74.29% (p=0.000 n=10)

                  │  baseline   │                 new                 │
                  │  heapB/ctx  │  heapB/ctx   vs base                │
HeapPerContext-16   69.83k ± 0%   88.31k ± 0%  +26.46% (p=0.000 n=10)
```

## gojs
```
goos: linux
goarch: amd64
pkg: github.com/botify-hq/cdf/gojs
cpu: Intel(R) Core(TM) Ultra 9 285H
                                │  baseline   │                 new                 │
                                │   sec/op    │   sec/op     vs base                │
V8_Cleanup/QuerySelectors-16      140.9µ ± 2%   142.5µ ± 1%        ~ (p=0.075 n=10)
V8_Cleanup/ReadAttributes-16      139.2µ ± 2%   136.1µ ± 1%   -2.22% (p=0.002 n=10)
V8_Cleanup/WriteAttributes-16     149.8µ ± 2%   151.0µ ± 5%   +0.81% (p=0.015 n=10)
V8_Cleanup/DomMutation-16         155.3µ ± 2%   151.2µ ± 3%   -2.61% (p=0.002 n=10)
V8_Cleanup/TreeNavigation-16      232.8µ ± 1%   222.1µ ± 2%   -4.60% (p=0.002 n=10)
V8_Cleanup/MixedRealistic-16      250.9µ ± 2%   259.9µ ± 1%   +3.58% (p=0.000 n=10)
V8_ColdStart/QuerySelectors-16    1.796m ± 1%   1.572m ± 1%  -12.46% (p=0.000 n=10)
V8_ColdStart/ReadAttributes-16    1.808m ± 1%   1.565m ± 1%  -13.46% (p=0.000 n=10)
V8_ColdStart/WriteAttributes-16   1.834m ± 1%   1.573m ± 1%  -14.21% (p=0.000 n=10)
V8_ColdStart/DomMutation-16       1.859m ± 1%   1.599m ± 2%  -13.94% (p=0.000 n=10)
V8_ColdStart/TreeNavigation-16    1.960m ± 1%   1.715m ± 2%  -12.51% (p=0.000 n=10)
V8_ColdStart/MixedRealistic-16    2.003m ± 1%   1.741m ± 2%  -13.08% (p=0.000 n=10)
V8_CleanupOnly-16                 1.597µ ± 1%   1.812µ ± 2%  +13.43% (p=0.000 n=10)
geomean                           362.3µ        341.4µ        -5.75%

                                │   baseline   │                  new                  │
                                │     B/op     │     B/op      vs base                 │
V8_Cleanup/QuerySelectors-16      77.92Ki ± 0%   77.92Ki ± 0%       ~ (p=0.440 n=10)
V8_Cleanup/ReadAttributes-16      81.78Ki ± 0%   81.78Ki ± 0%       ~ (p=0.837 n=10)
V8_Cleanup/WriteAttributes-16     131.4Ki ± 0%   131.4Ki ± 0%       ~ (p=1.000 n=10)
V8_Cleanup/DomMutation-16         164.3Ki ± 0%   164.2Ki ± 0%  -0.00% (p=0.009 n=10)
V8_Cleanup/TreeNavigation-16      150.7Ki ± 0%   150.7Ki ± 0%       ~ (p=1.000 n=10)
V8_Cleanup/MixedRealistic-16      113.3Ki ± 0%   113.3Ki ± 0%       ~ (p=0.509 n=10)
V8_ColdStart/QuerySelectors-16    82.95Ki ± 0%   83.81Ki ± 0%  +1.04% (p=0.000 n=10)
V8_ColdStart/ReadAttributes-16    86.81Ki ± 0%   87.67Ki ± 0%  +0.99% (p=0.000 n=10)
V8_ColdStart/WriteAttributes-16   136.4Ki ± 0%   137.3Ki ± 0%  +0.63% (p=0.000 n=10)
V8_ColdStart/DomMutation-16       169.5Ki ± 0%   170.3Ki ± 0%  +0.51% (p=0.000 n=10)
V8_ColdStart/TreeNavigation-16    155.8Ki ± 0%   156.6Ki ± 0%  +0.55% (p=0.000 n=10)
V8_ColdStart/MixedRealistic-16    118.3Ki ± 0%   119.2Ki ± 0%  +0.72% (p=0.000 n=10)
V8_CleanupOnly-16                   320.0 ± 0%     320.0 ± 0%       ~ (p=1.000 n=10) ¹
geomean                           74.69Ki        74.95Ki       +0.34%
¹ all samples are equal

                                │  baseline   │                 new                  │
                                │  allocs/op  │  allocs/op   vs base                 │
V8_Cleanup/QuerySelectors-16      1.363k ± 0%   1.363k ± 0%       ~ (p=1.000 n=10) ¹
V8_Cleanup/ReadAttributes-16      1.112k ± 0%   1.112k ± 0%       ~ (p=1.000 n=10) ¹
V8_Cleanup/WriteAttributes-16     1.353k ± 0%   1.353k ± 0%       ~ (p=1.000 n=10) ¹
V8_Cleanup/DomMutation-16         1.430k ± 0%   1.430k ± 0%       ~ (p=1.000 n=10) ¹
V8_Cleanup/TreeNavigation-16      2.933k ± 0%   2.933k ± 0%       ~ (p=1.000 n=10) ¹
V8_Cleanup/MixedRealistic-16      2.021k ± 0%   2.021k ± 0%       ~ (p=1.000 n=10) ¹
V8_ColdStart/QuerySelectors-16    1.549k ± 0%   1.603k ± 0%  +3.49% (p=0.000 n=10)
V8_ColdStart/ReadAttributes-16    1.298k ± 0%   1.352k ± 0%  +4.16% (p=0.000 n=10)
V8_ColdStart/WriteAttributes-16   1.539k ± 0%   1.593k ± 0%  +3.51% (p=0.000 n=10)
V8_ColdStart/DomMutation-16       1.616k ± 0%   1.670k ± 0%  +3.34% (p=0.000 n=10)
V8_ColdStart/TreeNavigation-16    3.119k ± 0%   3.173k ± 0%  +1.73% (p=0.000 n=10)
V8_ColdStart/MixedRealistic-16    2.207k ± 0%   2.261k ± 0%  +2.45% (p=0.000 n=10)
V8_CleanupOnly-16                  11.00 ± 0%    11.00 ± 0%       ~ (p=1.000 n=10) ¹
geomean                           1.156k        1.172k       +1.42%
¹ all samples are equal
```

## Endurance (100 000 itérations)
| Test | Version | RSS début → fin | Croissance 2e moitié | Verdict |
|---|---|---|---|---|
| v8go TestSoakCleanup | baseline | 58 → 72 MiB | 2.09 % | PASS |
| v8go TestSoakCleanup | new | 55 → 104 MiB | 0.90 % | PASS |
| gojs TestSoakV8Cleanup | baseline | 36 → 45 MiB (après GC V8 complet) | 5.24 % | FAIL |
| gojs TestSoakV8Cleanup | new | 48 → 58 MiB (après GC V8 complet) | 1.00 % | PASS |

La baseline gojs échoue de nouveau, juste au-dessus du seuil de 5 % (6.58 % lors de la Task 14, 4.90 % lors de la Task 13). C'est la même forme « plateau à marches » qu'en Task 13, et le seuil n'est pas modifié. La nouvelle version passe les deux soaks. `bench/run.sh` distingue désormais un échec de soak (`--- FAIL`) d'une erreur de compilation ou d'installation : ici, c'est bien un `--- FAIL`.

## Mémoire
| Mesure | baseline | new | Écart |
|---|---|---|---|
| RSS pic par worker gojs (max de `peak_rss`, soak gojs 100k) | 45.9 MiB | 59.4 MiB | +13.5 MiB (+29 %) |
| RSS plateau du soak v8go (1 isolate + 1 contexte réutilisés) | ~72 MiB | ~104 MiB | +32 MiB |
| heapB/iso (tas V8 utilisé, isolate neuf + 1 contexte) | 744.6k | 191.4k | -74 % |
| heapB/ctx (tas V8 ajouté par contexte supplémentaire) | 69.83k | 88.31k | +26 % |

Retirer le shim ne réduit pas le RSS (Task 14 : 104 MiB et 58 MiB avec ou sans shim ; même chose ici). Le surcoût vient de V8 15 lui-même (tas, cage sandbox/pointer compression, code). Il faut prévoir environ +10 à +14 MiB par process worker gojs. heapB/* est lu sans GC préalable : valeurs indicatives. La baisse de heapB/iso tient probablement à la comptabilité de V8 15 (espace read-only hors de UsedHeapSize), ce qui n'est pas vérifié.

## Régressions restantes > 5 % (p < 0.05)

- **CallGoFromJS1000 : +20.9 %** dans cette passe (baseline 744.2 µs ± 2 %, new 899.4 µs ± 3 %). L'A/B sans shim de la Task 14, une autre passe, donnait +18.3 %. L'écart entre les deux passes est donc d'environ 3 points. Le shim n'explique pas cette régression : elle persiste sans lui. Cause probable, non mesurée : le chemin callback JS→Go de V8 15 et du bridge (par argument, un `std::vector` sur le tas au lieu d'un VLA et un `unordered_map` de suivi au lieu d'un `vector`, plus un retour Go à deux valeurs).
- **Cleanup1000Values : +123.8 %**, et **gojs V8_CleanupOnly : +13.4 %**. C'est le coût voulu du correctif de la Task 12, sans lequel la mémoire fuit : `Isolate.Cleanup` pompe les tâches de la plateforme V8. S'ajoute le parcours de l'`unordered_map`. Ce coût est payé une fois par page gojs, hors des benchmarks par appel.
- **JSObjects : +10.7 % (±202 %)** : non confirmé. Le benchmark est bimodal en new, à cause d'un GC majeur pendant certains échantillons (voir Task 14).
- **B/op Go** (+32 octets sur NewIsolate/HeapPerIsolate/StartupWithLodash) : allocations Go marginales, sans impact.

CallJSFromGo (+2.8 %) et ValueToGoString (+2.6 %) restent sous le seuil de 5 %.

## Points revus depuis la Task 14

- **CompileLodashCold** : la Task 14 mesurait +19.5 %, avec un A/B sans shim non concluant (±121 %). Dans cette passe, la nouvelle version est à **-8.1 %**. La régression disparaît en même temps que le shim, mais aucun A/B dans une même passe ne l'isole. **Cause de l'écart de la Task 14 : non établie.**
- **CompileLodashWithCodeCache** : -11.9 % (contre +21.4 % avec shim). C'est cohérent avec l'A/B de la Task 14, où il revenait au niveau de la baseline sans shim.
- **gojs V8_Cleanup/*** : au niveau de la baseline (-4.6 % à +3.6 %, QuerySelectors n.s.). **V8_ColdStart/*** : -12.5 à -14.2 %.

Les preuves brutes de l'analyse du shim (strace, défauts de page, callgrind) sont dans le rapport de la Task 14. Le retrait lui-même (inventaire des 5 plateformes, méthode) est décrit dans le rapport de la Task 24 (`.superpowers/sdd/2026-10-06-v8-upgrade/task-24-report.md`).
