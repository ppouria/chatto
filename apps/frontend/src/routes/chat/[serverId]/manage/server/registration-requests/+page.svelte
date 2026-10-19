<script lang="ts">
  import { errorMessage, toastError } from '$lib/utils/errorMessage';
  import {
    createRegistrationRequestAPI,
    type RegistrationRequest
  } from '$lib/api/registrationRequests';
  import {
    ConfirmDialog,
    Hint,
    LoadingFog,
    Panel,
    PaneContent,
    PaneHeader,
    PageTitle
  } from '$lib/ui';
  import { Button } from '$lib/ui/form';
  import { adminQueryKeys } from '$lib/query/admin';
  import { createInfiniteQuery, createMutation, queryClient } from '$lib/query/client';
  import { useServerScope } from '$lib/state/server/scope.svelte';
  import { m } from '$lib/i18n/messages';
  import { getLocale } from '$lib/i18n/runtime';
  import { toast } from '$lib/ui/toast';
  import { formatDateTime, timeFormatSettingsFor } from '$lib/utils/formatTime';

  const PAGE_SIZE = 20;
  const serverScope = useServerScope();
  const userSettings = $derived(
    timeFormatSettingsFor(serverScope.store.currentUser.user?.settings)
  );
  const activeLocale = $derived(getLocale());
  let denyTarget = $state<RegistrationRequest | null>(null);

  const requestsQuery = createInfiniteQuery(() => {
    const serverId = serverScope.serverId;
    const connection = serverScope.connection;
    return {
      queryKey: adminQueryKeys.registrationRequests(serverId, connection),
      queryFn: ({ pageParam, signal }) =>
        connection.getAPI(createRegistrationRequestAPI).list(pageParam, PAGE_SIZE, { signal }),
      initialPageParam: 0,
      getNextPageParam: (lastPage, _pages, lastPageParam) =>
        lastPage.hasMore && lastPage.requests.length > 0
          ? lastPageParam + lastPage.requests.length
          : undefined
    };
  });

  const requests = $derived((requestsQuery.data?.pages ?? []).flatMap((page) => page.requests));

  async function refresh() {
    await queryClient.invalidateQueries({
      queryKey: adminQueryKeys.registrationRequests(serverScope.serverId, serverScope.connection)
    });
  }

  const approveMutation = createMutation(() => ({
    mutationFn: (request: RegistrationRequest) =>
      serverScope.connection.getAPI(createRegistrationRequestAPI).approve(request.id),
    onSuccess: async (login: string) => {
      toast.success(m('admin.registration_requests.approved', { login }));
      await refresh();
    },
    onError: (error) => toastError(error, m('admin.registration_requests.failed'))
  }));

  const denyMutation = createMutation(() => ({
    mutationFn: (request: RegistrationRequest) =>
      serverScope.connection.getAPI(createRegistrationRequestAPI).deny(request.id),
    onSuccess: async () => {
      denyTarget = null;
      toast.success(m('admin.registration_requests.denied'));
      await refresh();
    },
    onError: (error) => toastError(error, m('admin.registration_requests.failed'))
  }));
</script>

<PageTitle title={m('admin.registration_requests.title')} />

<PaneHeader
  title={m('admin.registration_requests.title')}
  subtitle={m('admin.registration_requests.subtitle')}
/>

<PaneContent>
  <Panel title={m('admin.registration_requests.title')} noPadding>
    {#if requestsQuery.isPending}
      <LoadingFog />
    {:else if requestsQuery.isError}
      <div class="p-5"><Hint tone="danger">{errorMessage(requestsQuery.error)}</Hint></div>
    {:else if requests.length === 0}
      <div class="p-5"><Hint>{m('admin.registration_requests.empty')}</Hint></div>
    {:else}
      <ul class="divide-y divide-border">
        {#each requests as request (request.id)}
          <li class="flex flex-col gap-3 p-5 sm:flex-row sm:items-start sm:justify-between">
            <div class="min-w-0 flex-1">
              <p class="font-semibold break-words">{request.login}</p>
              <p class="text-sm break-all text-muted">
                {m('admin.registration_requests.email')}: {request.email}
              </p>
              {#if request.note}
                <p class="mt-2 break-words whitespace-pre-wrap">{request.note}</p>
              {/if}
              <p class="mt-2 text-sm text-muted">
                {m('admin.registration_requests.submitted', {
                  time: formatDateTime(request.createdAt, userSettings, activeLocale)
                })}
              </p>
            </div>
            <div class="flex shrink-0 gap-2">
              <Button
                size="sm"
                loading={approveMutation.isPending && approveMutation.variables?.id === request.id}
                disabled={approveMutation.isPending || denyMutation.isPending}
                onclick={() => approveMutation.mutate(request)}
              >
                {m('admin.registration_requests.approve')}
              </Button>
              <Button
                size="sm"
                variant="danger"
                disabled={approveMutation.isPending || denyMutation.isPending}
                onclick={() => (denyTarget = request)}
              >
                {m('admin.registration_requests.deny')}
              </Button>
            </div>
          </li>
        {/each}
      </ul>
      {#if requestsQuery.hasNextPage}
        <div class="p-5">
          <Button
            variant="secondary"
            loading={requestsQuery.isFetchingNextPage}
            onclick={() => requestsQuery.fetchNextPage()}
          >
            {m('admin.registration_requests.load_more')}
          </Button>
        </div>
      {/if}
    {/if}
  </Panel>
</PaneContent>

{#if denyTarget}
  <ConfirmDialog
    title={m('admin.registration_requests.deny_title')}
    actionLabel={m('admin.registration_requests.deny')}
    loading={denyMutation.isPending}
    onconfirm={() => denyTarget && denyMutation.mutate(denyTarget)}
    onclose={() => (denyTarget = null)}
  >
    {m('admin.registration_requests.deny_description')}
  </ConfirmDialog>
{/if}
