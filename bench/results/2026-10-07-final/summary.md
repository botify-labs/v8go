# V8 9.0 (6f9829d) → V8 15.4 (branche de pré-publication) — résultats du 2026-10-07

Le code mesuré, celui de la branche de pré-publication, est celui des bridges épinglés à cette date. L'isolation du runtime C++ de V8, ajoutée ensuite, a été mesurée à part, à −0,05 % de geomean v8go (`../2026-10-07-cxx-isolation/`). Les correctifs postérieurs (garde-fous du pompage des tâches V8 dans `Isolate.Cleanup` et des callbacks, `tools/patches/0005-callback-guards.patch`) ont été mesurés à −0,75 % de geomean, non significatif, sur les benchmarks de callback (mesure séparée, non versionnée ici). Les chiffres ci-dessous valent donc pour la version publiée.

Machine : Intel Core Ultra 9 285H, 16 CPU, Docker linux/amd64 (WSL2), Go 1.27.1, gcc 12.2. La nouvelle version est compilée comme chez un consommateur (gcc + bridge précompilé, sans allocator shim, avec les optimisations du chemin JS→Go). COUNT=10.

**v8go** : `v8go-il-*.txt`, mesuré en alternant ancienne et nouvelle version (10 tours d'un passage `-test.count 1` chacun, l'ordre des deux versions changeant à chaque tour, `Cleanup1000Values` à part en `-test.benchtime 2000x`), pour neutraliser les perturbations de la machine. Le script de ce passage n'a pas été conservé : la section `v8go-il` de `bench/run.sh` (`tools/docker/dev.sh bench/run.sh v8go-il`) reproduit la procédure. Le premier passage non alterné (`v8go-benchstat.txt`) a été trop bruité côté nouvelle version (±33 à ±206 %) et n'est pas retenu.
**Endurance** : passage `bench/run.sh` du même jour.

## v8go (alterné, `v8go-il-benchstat.txt`)

| Benchmark | Ancienne | Nouvelle | Écart |
|---|---|---|---|
| CallGoFromJS1000 (1000 appels JS→Go) | 729 µs | 316 µs | **−56,7 %** |
| CallGoFromJS1000Cleanup (1000 appels JS→Go puis Cleanup, chronométré) | 409 µs | 358 µs | **−12,5 %** |
| CallJSFromGo | 836 ns | 740 ns | −11,4 % |
| RunScriptTrivial | 760 ns | 693 ns | −8,9 % |
| NewValueString | 344 ns | 296 ns | −14,0 % |
| ObjectSetGet | 1069 ns | 841 ns | −21,3 % |
| ValueToGoString | 1,39 µs | 1,42 µs | +2,1 % |
| NewIsolate | 896 µs | 854 µs | −4,8 % |
| NewContext | 189 µs | 122 µs | −35,8 % |
| JSONParse | 93,6 µs | 24,3 µs | −74,1 % |
| JSONStringify | 53,6 µs | 26,8 µs | −50,0 % |
| CompileLodashCold | 2,55 ms | 2,35 ms | −7,9 % |
| CompileLodashWithCodeCache | 466 µs | 410 µs | −12,0 % |
| StartupWithLodash | 5,01 ms | 3,86 ms | −23,0 % |
| JSRegex | 56,1 µs | 33,5 µs | −40,2 % |
| JSStrings | 539 µs | 436 µs | −19,1 % |
| JSArrays | 4,27 ms | 4,00 ms | −6,3 % |
| JSJSON | 1,37 ms | 0,56 ms | −59,4 % |
| JSObjects | 151 µs | 404 µs ± 59 % | +167 % (bimodal, voir plus bas) |
| Cleanup1000Values | 9,6 µs | 13,4 µs | +39,3 % |
| **geomean** | 113,0 µs | 87,8 µs | **−22,3 %** |

Toutes les lignes sont significatives (p < 0.05, n=10).

## Endurance (100 000 itérations)

| Test | Ancienne | Nouvelle |
|---|---|---|
| v8go TestSoakCleanup (RSS) | 59 → 72 MiB, +2,58 % sur la 2e moitié, PASS | 54 → 103 MiB, +0,62 %, PASS |

## Lecture

- **Chemin JS→Go** : −57 % sur des appels en rafale (`CallGoFromJS1000`), −12,5 % quand chaque série d'appels est suivie d'un `Cleanup` (`CallGoFromJS1000Cleanup`). La régression initiale de +21 % est résorbée.
- **Cleanup** : reste +39 % en microbenchmark (`Cleanup1000Values`). Le benchmark n'appelle que `ctx.Cleanup()`, qui ne vide pas la file des tâches V8 : l'écart vient de la libération des handles `Global` par V8 15 (`GlobalHandles::Destroy`, `../2026-10-07-callback/callback-perf.md`), payée une fois par `Cleanup`.
- **JSObjects** : bimodal dans la nouvelle version (±59 %). Hypothèse, non vérifiée : certains échantillons subissent un GC majeur de V8 15 pendant la mesure (10 000 objets créés par opération, valeurs retenues jusqu'au Cleanup). Le phénomène est déjà observé dans les campagnes précédentes (+10,7 % ± 202 %).
- **Mémoire** : environ +32 MiB de plateau par isolate de longue durée (soak v8go). C'est le coût fixe de V8 15, sans fuite.
