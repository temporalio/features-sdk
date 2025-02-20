package features

import (
	activity_basic_no_workflow_timeout "github.com/temporalio/features/features/activity/basic_no_workflow_timeout"

	activity_cancel_try_cancel "github.com/temporalio/features/features/activity/cancel_try_cancel"
	activity_retry_on_error "github.com/temporalio/features/features/activity/retry_on_error"
	bugs_go_activity_start_race "github.com/temporalio/features/features/bugs/go/activity_start_race"
	bugs_go_child_workflow_cancel_panic "github.com/temporalio/features/features/bugs/go/child_workflow_cancel_panic"
	build_id_versioning_activity_and_child_on_correct_version "github.com/temporalio/features/features/build_id_versioning/activity_and_child_on_correct_version"
	build_id_versioning_continues_as_new_on_correct_version "github.com/temporalio/features/features/build_id_versioning/continues_as_new_on_correct_version"
	build_id_versioning_only_appropriate_worker_gets_task "github.com/temporalio/features/features/build_id_versioning/only_appropriate_worker_gets_task"
	build_id_versioning_unversioned_worker_gets_unversioned_task "github.com/temporalio/features/features/build_id_versioning/unversioned_worker_gets_unversioned_task"
	build_id_versioning_unversioned_worker_no_task "github.com/temporalio/features/features/build_id_versioning/unversioned_worker_no_task"
	build_id_versioning_versions_added_while_worker_polling "github.com/temporalio/features/features/build_id_versioning/versions_added_while_worker_polling"
	child_workflow_result "github.com/temporalio/features/features/child_workflow/result"
	child_workflow_signal "github.com/temporalio/features/features/child_workflow/signal"
	client_http_proxy "github.com/temporalio/features/features/client/http_proxy"
	client_http_proxy_auth "github.com/temporalio/features/features/client/http_proxy_auth"
	continue_as_new_continue_as_same "github.com/temporalio/features/features/continue_as_new/continue_as_same"
	data_converter_binary "github.com/temporalio/features/features/data_converter/binary"
	data_converter_binary_protobuf "github.com/temporalio/features/features/data_converter/binary_protobuf"
	data_converter_codec "github.com/temporalio/features/features/data_converter/codec"
	data_converter_empty "github.com/temporalio/features/features/data_converter/empty"
	data_converter_failure "github.com/temporalio/features/features/data_converter/failure"
	data_converter_json "github.com/temporalio/features/features/data_converter/json"
	data_converter_json_protobuf "github.com/temporalio/features/features/data_converter/json_protobuf"
	eager_activity_non_remote_activities_worker "github.com/temporalio/features/features/eager_activity/non_remote_activities_worker"
	eager_workflow_successful_start "github.com/temporalio/features/features/eager_workflow/successful_start"
	query_successful_query "github.com/temporalio/features/features/query/successful_query"
	query_timeout_due_to_no_active_workers "github.com/temporalio/features/features/query/timeout_due_to_no_active_workers"
	query_unexpected_arguments "github.com/temporalio/features/features/query/unexpected_arguments"
	query_unexpected_query_type_name "github.com/temporalio/features/features/query/unexpected_query_type_name"
	query_unexpected_return_type "github.com/temporalio/features/features/query/unexpected_return_type"
	reset_reset_and_delete "github.com/temporalio/features/features/reset/reset_and_delete"
	schedule_backfill "github.com/temporalio/features/features/schedule/backfill"
	schedule_basic "github.com/temporalio/features/features/schedule/basic"
	schedule_cron "github.com/temporalio/features/features/schedule/cron"
	schedule_pause "github.com/temporalio/features/features/schedule/pause"
	schedule_trigger "github.com/temporalio/features/features/schedule/trigger"
	signal_external "github.com/temporalio/features/features/signal/external"
	telemetry_metrics "github.com/temporalio/features/features/telemetry/metrics"
	update_activities "github.com/temporalio/features/features/update/activities"
	update_async_accepted "github.com/temporalio/features/features/update/async_accepted"
	update_basic "github.com/temporalio/features/features/update/basic"
	update_client_interceptor "github.com/temporalio/features/features/update/client_interceptor"
	update_deduplication "github.com/temporalio/features/features/update/deduplication"
	update_non_durable_reject "github.com/temporalio/features/features/update/non_durable_reject"
	update_non_durable_reject_with_events "github.com/temporalio/features/features/update/non_durable_reject_with_events"
	update_self "github.com/temporalio/features/features/update/self"
	update_task_failure "github.com/temporalio/features/features/update/task_failure"
	update_validation_replay "github.com/temporalio/features/features/update/validation_replay"
	update_worker_restart "github.com/temporalio/features/features/update/worker_restart"
	harness "github.com/temporalio/features/harness/go/harness"
)

func init() {
	// Please keep list in alphabetical order
	harness.MustRegisterFeatures(
		activity_basic_no_workflow_timeout.Feature,
		activity_cancel_try_cancel.Feature,
		activity_retry_on_error.Feature,
		bugs_go_activity_start_race.Feature,
		bugs_go_child_workflow_cancel_panic.Feature,
		build_id_versioning_activity_and_child_on_correct_version.Feature,
		build_id_versioning_continues_as_new_on_correct_version.Feature,
		build_id_versioning_only_appropriate_worker_gets_task.Feature,
		build_id_versioning_unversioned_worker_gets_unversioned_task.Feature,
		build_id_versioning_unversioned_worker_no_task.Feature,
		build_id_versioning_versions_added_while_worker_polling.Feature,
		child_workflow_result.Feature,
		child_workflow_signal.Feature,
		client_http_proxy.Feature,
		client_http_proxy_auth.Feature,
		continue_as_new_continue_as_same.Feature,
		data_converter_binary_protobuf.Feature,
		data_converter_binary.Feature,
		data_converter_codec.Feature,
		data_converter_empty.Feature,
		data_converter_failure.Feature,
		data_converter_json_protobuf.Feature,
		data_converter_json.Feature,
		eager_activity_non_remote_activities_worker.Feature,
		eager_workflow_successful_start.Feature,
		query_successful_query.Feature,
		query_timeout_due_to_no_active_workers.Feature,
		query_unexpected_arguments.Feature,
		query_unexpected_query_type_name.Feature,
		query_unexpected_return_type.Feature,
		reset_reset_and_delete.Feature,
		schedule_backfill.Feature,
		schedule_basic.Feature,
		schedule_cron.Feature,
		schedule_pause.Feature,
		schedule_trigger.Feature,
		signal_external.Feature,
		telemetry_metrics.Feature,
		update_activities.Feature,
		update_async_accepted.Feature,
		update_basic.Feature,
		update_deduplication.Feature,
		update_client_interceptor.Feature,
		update_non_durable_reject.Feature,
		update_non_durable_reject_with_events.Feature,
		update_self.Feature,
		update_task_failure.Feature,
		update_validation_replay.Feature,
		update_worker_restart.Feature,
	)
}
