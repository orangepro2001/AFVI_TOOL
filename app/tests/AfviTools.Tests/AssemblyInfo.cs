// Loc is a process wide singleton, so tests that switch language must not run in parallel.
[assembly: CollectionBehavior(DisableTestParallelization = true)]
