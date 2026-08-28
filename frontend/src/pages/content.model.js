function number_or_default(value, fallback) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function first_non_empty(...values) {
  for (const value of values) {
    if (value !== undefined && value !== null && value !== "") {
      return value;
    }
  }
  return "";
}

function content_detail_href(content) {
  const id = String(first_non_empty(content && content.id, content && content.ID)).trim();
  return id ? `/content/detail?id=${encodeURIComponent(id)}` : "";
}

function normalize_content_list_response(data, fallbackPage, fallbackSize) {
  const source = data && typeof data === "object" ? data : {};
  const list = Array.isArray(source.list)
    ? source.list
    : Array.isArray(source.List)
      ? source.List
      : [];
  return {
    list,
    total: Math.max(
      0,
      number_or_default(
        typeof source.total !== "undefined" ? source.total : source.Total,
        list.length,
      ),
    ),
    page: Math.max(
      1,
      number_or_default(source.page || source.Page, fallbackPage),
    ),
    page_size: Math.max(
      1,
      number_or_default(
        source.page_size || source.pageSize || source.PageSize,
        fallbackSize,
      ),
    ),
  };
}

function normalize_content_account(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  return {
    id: first_non_empty(source.id, source.ID),
    external_id: first_non_empty(source.external_id, source.ExternalID),
    alias: first_non_empty(source.alias, source.Alias),
    nickname: first_non_empty(
      source.nickname,
      source.Nickname,
      source.name,
      source.Name,
    ),
    avatar_url: first_non_empty(source.avatar_url, source.AvatarURL),
    profile_url: first_non_empty(source.profile_url, source.ProfileURL),
  };
}

function normalize_content_item(raw) {
  const source = raw && typeof raw === "object" ? raw : {};
  const accounts_source = Array.isArray(source.accounts)
    ? source.accounts
    : Array.isArray(source.Accounts)
      ? source.Accounts
      : [];
  const tasks = Array.isArray(source.download_tasks)
    ? source.download_tasks
    : Array.isArray(source.DownloadTasks)
      ? source.DownloadTasks
      : [];
  const file_count = Math.max(
    0,
    number_or_default(
      first_non_empty(source.file_count, source.fileCount, source.FileCount),
      0,
    ),
  );

  return {
    ...source,
    id: first_non_empty(source.id, source.ID),
    platform_id: first_non_empty(source.platform_id, source.PlatformID),
    platform_name: first_non_empty(source.platform_name, source.PlatformName),
    content_type: first_non_empty(
      source.content_type,
      source.ContentType,
      source.type,
      source.Type,
    ),
    content_subtype: first_non_empty(
      source.content_subtype,
      source.ContentSubtype,
      source.subtype,
      source.Subtype,
    ),
    title: first_non_empty(
      source.title,
      source.Title,
      source.description,
      source.Description,
    ),
    description: first_non_empty(source.description, source.Description),
    url: first_non_empty(
      source.source_url,
      source.SourceURL,
      source.url,
      source.URL,
      source.content_url,
      source.ContentURL,
    ),
    cover_url: first_non_empty(
      source.cover_url,
      source.CoverURL,
      source.coverUrl,
    ),
    publish_time: number_or_default(
      first_non_empty(source.publish_time, source.PublishTime),
      0,
    ),
    view_count: Math.max(
      0,
      number_or_default(first_non_empty(source.view_count, source.ViewCount), 0),
    ),
    like_count: Math.max(
      0,
      number_or_default(first_non_empty(source.like_count, source.LikeCount), 0),
    ),
    comment_count: Math.max(
      0,
      number_or_default(
        first_non_empty(source.comment_count, source.CommentCount),
        0,
      ),
    ),
    share_count: Math.max(
      0,
      number_or_default(first_non_empty(source.share_count, source.ShareCount), 0),
    ),
    collect_count: Math.max(
      0,
      number_or_default(
        first_non_empty(source.collect_count, source.CollectCount),
        0,
      ),
    ),
    accounts: accounts_source.map(normalize_content_account),
    download_tasks: tasks,
    file_count,
  };
}

function content_platform_name(content) {
  if (content.platform_name) {
    return content.platform_name;
  }
  return (
    window.PLATFORM_NAMES[content.platform_id] ||
    content.platform_id ||
    "未知平台"
  );
}

function content_type_label(value, subtypeValue) {
  const type = String(value || "")
    .trim()
    .toLowerCase();
  const subtype = String(subtypeValue || "")
    .trim()
    .toLowerCase();
  return (
    window.CONTENT_TYPE_NAMES[subtype] ||
    window.CONTENT_TYPE_NAMES[type] ||
    subtype ||
    type ||
    "内容"
  );
}

function content_statistics(content) {
  const source = content && typeof content === "object" ? content : {};
  const tasks = Array.isArray(source.download_tasks)
    ? source.download_tasks
    : [];
  const statistics = {
    total_tasks: tasks.length,
    in_progress: 0,
    failed: 0,
    files: Math.max(0, number_or_default(source.file_count, 0)),
  };
  tasks.forEach((task) => {
    const status = task.status;
    if ([2, 3, 4].includes(status)) {
      statistics.in_progress += 1;
    } else if ([6, 7].includes(status)) {
      statistics.failed += 1;
    }
  });
  return statistics;
}

function format_content_count(value) {
  const count = Math.max(0, number_or_default(value, 0));
  return new Intl.NumberFormat("zh-CN", { notation: "compact" }).format(count);
}

function ContentViewModel(props) {
  const PAGE_SIZE_DEFAULT = 24;
  const contents_ = refarr([]);
  const total_ = ref(0);
  const page_ = ref(1);
  const page_size_ = ref(PAGE_SIZE_DEFAULT);
  const keyword_ = ref("");
  const content_type_ = ref("");
  const scope_ = ref("task");
  const sort_key_ = ref("publish_time:desc");
  const min_like_count_ = ref("");
  const selected_content_ids_ = refarr([]);
  const batch_downloading_ = ref(false);
  const batch_download_message_ = ref("");
  const batch_download_error_ = ref("");
  const initial_ = ref(true);
  const loading_ = ref(false);
  const error_ = ref("");
  const detail_id_ = ref("");
  let request_sequence = 0;

  const methods = {};
  const ui = {
    input_keyword$: new Timeless.vm.InputCore({
      defaultValue: keyword_.value,
      placeholder: "搜索标题或描述",
      type: "search",
      allowClear: true,
      onChange(value) {
        set_keyword(value);
      },
    }),
    input_min_like_count$: new Timeless.vm.InputCore({
      defaultValue: min_like_count_.value,
      placeholder: "最低点赞",
      type: "number",
      allowClear: true,
      onChange(value) {
        min_like_count_.as(String(value ?? ""));
      },
    }),
    checkbox_all$: new Timeless.vm.CheckboxCore({
      checked: scope_.value === "all",
      onChange(value) {
        set_all_scope(value);
      },
    }),
    select_sort$: new Timeless.vm.SelectCore({
      defaultValue: sort_key_.value,
      placeholder: "排序",
      options: [
        new Timeless.vm.SelectItemCore({
          label: "最新发布",
          value: "publish_time:desc",
        }),
        new Timeless.vm.SelectItemCore({
          label: "最早发布",
          value: "publish_time:asc",
        }),
        new Timeless.vm.SelectItemCore({
          label: "点赞最多",
          value: "like_count:desc",
        }),
        new Timeless.vm.SelectItemCore({
          label: "最近抓取",
          value: "created_at:desc",
        }),
      ],
      onChange(value) {
        sort_key_.as(String(value || "publish_time:desc"));
        load(1);
      },
    }),
    select_content_type$: new Timeless.vm.SelectCore({
      defaultValue: "",
      placeholder: "全部类型",
      options: [
        new Timeless.vm.SelectItemCore({ label: "全部类型", value: "" }),
        new Timeless.vm.SelectItemCore({ label: "视频", value: "video" }),
        new Timeless.vm.SelectItemCore({
          label: "短视频",
          value: "short_video",
        }),
        new Timeless.vm.SelectItemCore({ label: "图集", value: "image_set" }),
        new Timeless.vm.SelectItemCore({ label: "文章", value: "article" }),
        new Timeless.vm.SelectItemCore({ label: "小说", value: "novel" }),
        new Timeless.vm.SelectItemCore({ label: "音频", value: "audio" }),
        new Timeless.vm.SelectItemCore({ label: "直播", value: "live" }),
      ],
      onChange(value) {
        content_type_.as(String(value || ""));
        load(1);
      },
    }),
    btn_search$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "primary",
    }),
    btn_refresh$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "outline",
      onClick() {
        return load(1);
      },
    }),
    btn_retry$: new Timeless.vm.ButtonCore({
      disabled: loading_.value,
      variant: "primary",
      onClick() {
        return load(page_.value);
      },
    }),
    btn_download_selected$: new Timeless.vm.ButtonCore({
      disabled: true,
      variant: "primary",
      onClick() {
        return methods.downloadSelected();
      },
    }),
    btn_clear_selection$: new Timeless.vm.ButtonCore({
      variant: "outline",
      onClick() {
        methods.clearSelection();
      },
    }),
    contentDetailDrawer$: new Timeless.vm.DialogCore({
      title: "内容详情",
      closeable: true,
      footer: false,
    }),
  };

  keyword_.subscribe({
    onChange(value) {
      if (ui.input_keyword$.value !== value) {
        ui.input_keyword$.setValue(value, { silence: true });
      }
    },
  });
  min_like_count_.subscribe({
    onChange(value) {
      if (ui.input_min_like_count$.value !== value) {
        ui.input_min_like_count$.setValue(value, { silence: true });
      }
    },
  });
  scope_.subscribe({
    onChange(value) {
      const checked = value === "all";
      if (ui.checkbox_all$.checked !== checked) {
        ui.checkbox_all$.setValue(checked, { silence: true });
      }
    },
  });
  loading_.subscribe({
    onChange(loading) {
      [ui.btn_search$, ui.btn_refresh$, ui.btn_retry$].forEach((button) => {
        if (loading) {
          button.disable();
        } else {
          button.enable();
        }
      });
    },
  });

  const list_request = new Timeless.kit.RequestCore(
    (params) => window.request.get("/api/content/list", params),
    {
      client: props.client,
      process(response) {
        if (response.error) {
          return Timeless.Result.Err(response.error);
        }
        return Timeless.Result.Ok(
          normalize_content_list_response(
            response.data,
            page_.value,
            page_size_.value,
          ),
        );
      },
    },
  );
  const batch_download_request = new Timeless.kit.RequestCore(
    (body) => window.request.post("/api/content/download", body),
    { client: props.client },
  );

  const page_count_ = combine(
    { total: total_, pageSize: page_size_ },
    (state) =>
      Math.max(1, Math.ceil(state.total / Math.max(1, state.pageSize))),
  );
  const range_text_ = combine(
    {
      total: total_,
      page: page_,
      pageSize: page_size_,
      count: computed(contents_, (contents) => contents.length),
    },
    (state) => {
      if (!state.total || !state.count) {
        return `共 ${state.total || 0} 条`;
      }
      const start = (state.page - 1) * state.pageSize + 1;
      return `第 ${start}-${start + state.count - 1} 条，共 ${state.total} 条`;
    },
  );
  const selected_content_count_ = computed(
    selected_content_ids_,
    (ids) => (ids || []).length,
  );
  const loaded_content_selection_ = combine(
    { contents: contents_, selected_ids: selected_content_ids_ },
    (state) => {
      const ids = (state.contents || [])
        .map((content) => String((content && content.id) || "").trim())
        .filter(Boolean);
      const selected_ids = state.selected_ids || [];
      const selected = ids.filter((id) => selected_ids.includes(id)).length;
      return {
        total: ids.length,
        selected,
        checked: ids.length > 0 && selected === ids.length,
        indeterminate: selected > 0 && selected < ids.length,
      };
    },
  );
  const selection_visible_ = computed(
    selected_content_count_,
    (count) => count > 0,
  );
  const list_status_ = combine(
    {
      initial: initial_,
      error: error_,
      contents: contents_,
    },
    (state) => {
      if (state.initial) return "initial";
      if (state.error) return "error";
      return state.contents.length > 0 ? "normal" : "empty";
    },
  );

  selected_content_count_.subscribe({
    onChange(count) {
      const disabled = count <= 0 || batch_downloading_.value;
      if (disabled) ui.btn_download_selected$.disable();
      else ui.btn_download_selected$.enable();
    },
  });
  batch_downloading_.subscribe({
    onChange(loading) {
      ui.btn_download_selected$.setLoading(Boolean(loading));
      if (loading || selected_content_count_.value <= 0) {
        ui.btn_download_selected$.disable();
      } else {
        ui.btn_download_selected$.enable();
      }
    },
  });

  function sort_params() {
    const [sort_by, sort_order] = String(sort_key_.value || "publish_time:desc").split(":");
    return {
      sort_by: sort_by || "publish_time",
      sort_order: sort_order || "desc",
    };
  }

  async function load(targetPage = page_.value) {
    const sequence = ++request_sequence;
    const requestedPage = Math.max(1, Number(targetPage) || 1);
    loading_.as(true);

    const params = {
      page: requestedPage,
      page_size: page_size_.value,
      scope: scope_.value,
      ...sort_params(),
    };
    const keyword = String(keyword_.value || "").trim();
    const contentType = String(content_type_.value || "").trim();
    const minLikeCount = Number(min_like_count_.value);
    if (keyword) {
      params.keyword = keyword;
    }
    if (contentType) {
      params.content_type = contentType;
    }
    if (Number.isFinite(minLikeCount) && minLikeCount > 0) {
      params.min_like_count = Math.floor(minLikeCount);
    }

    const result = await list_request.run(params);
    if (sequence !== request_sequence) {
      return result;
    }
    if (result.error) {
      error_.as(result.error.message || String(result.error));
      loading_.as(false);
      initial_.as(false);
      return result;
    }

    const data = result.data;
    error_.as("");
    contents_.as(data.list.map(normalize_content_item), { reset: true });
    total_.as(data.total);
    page_.as(data.page);
    page_size_.as(data.page_size);
    loading_.as(false);
    initial_.as(false);
    return result;
  }

  function visible_content_ids() {
    return (contents_.value || [])
      .map((content) => String((content && content.id) || "").trim())
      .filter(Boolean);
  }

  function set_loaded_contents_selected(selected) {
    const visible_ids = visible_content_ids();
    if (selected) {
      const ids = [...(selected_content_ids_.value || [])];
      visible_ids.forEach((id) => {
        if (!ids.includes(id)) ids.push(id);
      });
      selected_content_ids_.as(ids);
      return;
    }
    const visible = new Set(visible_ids);
    selected_content_ids_.as(
      (selected_content_ids_.value || []).filter((id) => !visible.has(id)),
    );
  }

  function toggle_loaded_contents_selected() {
    set_loaded_contents_selected(!loaded_content_selection_.value.checked);
  }

  function content_selection_state(content) {
    const id = String((content && content.id) || "").trim();
    return computed(selected_content_ids_, (ids) => ({
      checked: Boolean(id && (ids || []).includes(id)),
      indeterminate: false,
    }));
  }

  function toggle_content_selected(content) {
    const id = String((content && content.id) || "").trim();
    if (!id) return;
    const ids = [...(selected_content_ids_.value || [])];
    const index = ids.indexOf(id);
    if (index >= 0) ids.splice(index, 1);
    else ids.push(id);
    selected_content_ids_.as(ids);
  }

  function clear_selection() {
    selected_content_ids_.as([]);
    batch_download_message_.as("");
    batch_download_error_.as("");
  }

  async function download_selected() {
    if (batch_downloading_.value) return null;
    const ids = [...(selected_content_ids_.value || [])];
    if (ids.length === 0) return null;

    batch_downloading_.as(true);
    batch_download_message_.as("");
    batch_download_error_.as("");
    const request_result = await batch_download_request.run({ content_ids: ids });
    batch_downloading_.as(false);
    if (request_result.error) {
      batch_download_error_.as(
        request_result.error.message || String(request_result.error),
      );
      return request_result;
    }

    const data = request_result.data || {};
    const items = Array.isArray(data.items) ? data.items : [];
    const completed_ids = new Set(
      items
        .filter((item) => item && item.success)
        .map((item) => String(item.content_id || "").trim())
        .filter(Boolean),
    );
    selected_content_ids_.as(
      ids.filter((id) => !completed_ids.has(String(id))),
    );
    const created = Math.max(0, number_or_default(data.created, 0));
    const skipped = Math.max(0, number_or_default(data.skipped, 0));
    const failed = Math.max(0, number_or_default(data.failed, 0));
    batch_download_message_.as(
      [
        created > 0 ? `已创建 ${created} 个任务` : "",
        skipped > 0 ? `已跳过 ${skipped} 个已有任务` : "",
        failed > 0 ? `${failed} 个失败` : "",
      ]
        .filter(Boolean)
        .join("，") || "批量下载已处理",
    );
    if (failed > 0) {
      batch_download_error_.as("失败项已保留选中，可再次尝试");
    }
    await load(page_.value);
    return request_result;
  }

  function set_keyword(value) {
    keyword_.as(String(value || ""));
  }

  function set_all_scope(value) {
    scope_.as(value ? "all" : "task");
    return load(1);
  }

  Object.assign(methods, {
    ready() {
      return load(1);
    },
    refresh() {
      return load(1);
    },
    search() {
      return load(1);
    },
    setKeyword: set_keyword,
    previousPage() {
      if (page_.value <= 1 || loading_.value) {
        return null;
      }
      return load(page_.value - 1);
    },
    nextPage() {
      if (page_.value >= page_count_.value || loading_.value) {
        return null;
      }
      return load(page_.value + 1);
    },
    openSource(content) {
      if (!content || !content.url) {
        return;
      }
      window.open(content.url, "_blank", "noopener,noreferrer");
    },
    openDetail(content) {
      const id = String(
        first_non_empty(content && content.id, content && content.ID),
      ).trim();
      if (!id) return;
      detail_id_.as(id);
      ui.contentDetailDrawer$.show();
    },
    toggleLoadedContentsSelected: toggle_loaded_contents_selected,
    contentSelectionState: content_selection_state,
    toggleContentSelected: toggle_content_selected,
    clearSelection: clear_selection,
    downloadSelected: download_selected,
    detailHref: content_detail_href,
    platformName: content_platform_name,
    typeLabel: content_type_label,
    statistics: content_statistics,
    formatCount: format_content_count,
    formatTime: window.format_time,
  });

  const state = {
    contents: contents_,
    total: total_,
    page: page_,
    page_size: page_size_,
    page_count: page_count_,
    range_text: range_text_,
    scope: scope_,
    sort_key: sort_key_,
    min_like_count: min_like_count_,
    selected_content_ids: selected_content_ids_,
    selected_content_count: selected_content_count_,
    loaded_content_selection: loaded_content_selection_,
    selection_visible: selection_visible_,
    batch_downloading: batch_downloading_,
    batch_download_message: batch_download_message_,
    batch_download_error: batch_download_error_,
    initial: initial_,
    status: list_status_,
    loading: loading_,
    error: error_,
    detail_id: detail_id_,
  };

  return { state, ui, methods };
}

export { ContentViewModel };
