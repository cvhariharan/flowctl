---
title: Flow Versions
description: View, compare, and switch saved flow definitions
---

Flowctl saves a version of each distinct flow definition when it is created, updated, or imported
from disk. Versions store the source in its original YAML or HUML format. The active version is
the definition used for new executions.

## Viewing and Comparing Versions

Open a flow and click the **Version** badge beside its name to see its history, newest first.
The active version is marked **Active**.

- Choose **View** to read a saved version's source.
- Choose **Compare with active** to see what changed between that version and the active one.
- Use **From**, **To**, and **Compare** to compare any two saved versions. The diff shows added,
  removed, and unchanged lines.

Viewing history, source, and diffs requires the same **View Config** permission as reading the flow
definition. Operators, reviewers, and namespace admins have this permission.

## Switching Versions

To roll back an edit, choose **Switch to this version** on a saved version, review the diff, and
confirm. Switching requires the **Update** permission, which namespace admins have.

Switching writes the saved source back to the flow's file and makes that version active. It reuses
the saved version number. Saving or importing identical source also reuses its version.

!!! note
      Switching updates the flow file on disk. If you deploy definitions from version control,
      update that source too so a later deployment does not restore the unwanted definition.

## Executions and Schedules

Each new execution is pinned to the version it was prepared with. Updating or switching a flow
does not change running executions or one-off executions already queued with **Run Later**.

The execution page shows its pinned **Version**. **Retry** and re-running from an action use that
version and the existing inputs. Actions outside the re-run set keep their results and outputs.
Approval details also identify the execution's version, so reviewers can see which definition they are approving.
