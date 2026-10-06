# Fork Botify de v8go

Ce dépôt est un instantané de [tommie/v8go](https://github.com/tommie/v8go) (`deps/tommie_sha`)
renommé `github.com/botify-labs/v8go`. Les ajouts Botify sont listés dans `tools/botify-owned.txt` :

- **Bridge précompilé** : les `.cc` ne sont compilés qu'avec `-tags v8go_source`. Les consommateurs
  lient `deps/<os>_<arch>/libv8go.a` (`v8go.lib` sous Windows), produit par `tools/build_bridge.sh`.
  `tools/check_bridge.sh` vérifie qu'il correspond aux sources.
- `cleanup.*` : `Isolate.Cleanup()` / `Context.Cleanup()`. `Isolate.Cleanup()` exécute les tâches
  V8 de l'isolate ; le soak (`bench/soak_test.go`) ne passe que grâce au timer du memory reducer de
  V8 (≥ 8 s) : le soak de 100k itérations doit durer assez longtemps (~20 s) pour qu'il se déclenche.
- `bench/` : comparaison avec `v0.6.0-botify-baseline` (V8 9.0). `tools/docker/` : environnement de dev.

Mettre à jour V8 :
1. lancer `tools/docker/dev.sh 'tools/sync_tommie.sh <sha>'` (ou le workflow `botify-sync-upstream`) ;
2. lancer le workflow `botify-bridge` sur la branche ;
3. vérifier que `botify-ci` passe ;
4. fusionner, puis lancer `tools/pin_deps.sh <sha poussé>`.

Benchmarks : `tools/docker/dev.sh bench/run.sh`. Migration des consommateurs : `MIGRATION.md`.

## Piège clang-format

`clang-format` réécrit la première ligne `//go:build v8go_source` des fichiers `.cc` en
`// go:build v8go_source`. Le tag de build est alors ignoré **sans erreur** : les `.cc` sont compilés
dans le build normal, ou disparaissent du build `-tags v8go_source`. Après tout formatage, vérifier
la ligne 1 de chaque `.cc` (`head -1 *.cc`) : elle doit être exactement `//go:build v8go_source`.
