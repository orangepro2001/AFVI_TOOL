/* C ABI of the Go core (afvi-core.dll or libafvi-core.a). All strings are UTF-8.
 *
 *   afvi_init(config, selftestDir, allowNetworkWrite)
 *       config      path of config.json, or NULL for %AppData%\AFVI_TOOLS\config.json
 *       selftestDir NULL normally; a directory makes the core build throw-away local machines
 *                   T1/T2 there and hide the real network machine (used by -selftest)
 *       returns 0, or -1 on error
 *
 *   afvi_call(method, lang, argsJSON) -> malloc'ed JSON, release with afvi_free
 *       lang      "ko" | "zh-Hans" | "zh-Hant" | "en": the language of every sentence in the answer
 *       argsJSON  a JSON object or NULL/""
 *       answer    {"ok":true,"result":<value>}  or  {"ok":false,"error":"text","code":400}
 *
 *   Methods (arguments -> result):
 *     Health{} GetConfig{} ListModels{machine,node} Targets{machine,node} CheckName{name}
 *     PlanCopy{request} PlanRename{request}      request = ops.CopyRequest / RenameRequest, see docs
 *     StartJob{planId} GetJob{id} CancelJob{id}
 *     AddMachine{machine} UpdateMachine{name,machine} DeleteMachine{name} RestoreDefaults{}
 *     ExportExcel{}  (always an error "not implemented", the reserved entry)
 *     SelftestEnabled{} SelftestVerify{step}
 *
 * Thread safety: afvi_call may be called from any thread. A Go panic never crosses this boundary:
 * it becomes {"ok":false,...}. */
#ifndef AFVI_CORE_H
#define AFVI_CORE_H
#ifdef __cplusplus
extern "C" {
#endif
int   afvi_init(const char *config, const char *selftestDir, int allowNetworkWrite);
char *afvi_call(const char *method, const char *lang, const char *argsJSON);
void  afvi_free(char *p);
#ifdef __cplusplus
}
#endif
#endif
