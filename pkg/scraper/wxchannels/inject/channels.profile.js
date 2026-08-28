/**
 * @file 用户主页
 */
(() => {
  var my_username = "";
  var selector_panel = null;

  const metric_containers = [
    "favInfo",
    "feedInfo",
    "objectExtend",
    "statistics",
    "stats",
    "countInfo",
  ];

  function parse_metric(value) {
    if (typeof value === "number" && Number.isFinite(value)) {
      return value;
    }
    if (typeof value !== "string") {
      return null;
    }
    var raw = value.trim().replace(/[\s,，]/g, "").toLowerCase();
    if (!raw) {
      return null;
    }
    var multiplier = 1;
    var units = [
      ["千万", 10000000],
      ["百万", 1000000],
      ["亿", 100000000],
      ["万", 10000],
      ["m", 1000000],
      ["w", 10000],
      ["k", 1000],
    ];
    for (var i = 0; i < units.length; i += 1) {
      var unit = units[i][0];
      if (raw.endsWith(unit)) {
        multiplier = units[i][1];
        raw = raw.slice(0, -unit.length);
        break;
      }
    }
    var number = Number(raw);
    if (!Number.isFinite(number)) {
      return null;
    }
    return Math.round(number * multiplier);
  }

  function read_metric(feed, keys) {
    var candidates = [feed];
    for (var i = 0; i < metric_containers.length; i += 1) {
      var child = feed && feed[metric_containers[i]];
      if (child && typeof child === "object") {
        candidates.push(child);
      }
    }
    for (var c = 0; c < candidates.length; c += 1) {
      var source = candidates[c];
      for (var k = 0; k < keys.length; k += 1) {
        var key = keys[k];
        if (!Object.prototype.hasOwnProperty.call(source, key)) {
          continue;
        }
        var value = parse_metric(source[key]);
        if (value !== null) {
          return { known: true, value };
        }
      }
    }
    return { known: false, value: 0 };
  }

  function get_metrics(feed) {
    return {
      like: read_metric(feed, ["likeCount", "likeCountFmt", "likecount", "likecountfmt"]),
      comment: read_metric(feed, [
        "commentCount",
        "commentCountFmt",
        "commentcount",
        "commentcountfmt",
      ]),
      share: read_metric(feed, [
        "forwardCount",
        "forwardCountFmt",
        "shareCount",
        "shareCountFmt",
        "forwardcount",
        "forwardcountfmt",
      ]),
      collect: read_metric(feed, [
        "favCount",
        "favCountFmt",
        "favoriteCount",
        "collectCount",
        "favcount",
        "favcountfmt",
      ]),
    };
  }

  function format_metric(metric) {
    if (!metric.known) {
      return "—";
    }
    return new Intl.NumberFormat("zh-CN", { notation: "compact" }).format(
      metric.value,
    );
  }

  function get_feed_id(feed) {
    return String((feed && (feed.id || feed.objectId)) || "");
  }

  function get_feed_title(feed) {
    if (feed && feed.objectDesc && feed.objectDesc.description) {
      return feed.objectDesc.description;
    }
    return (feed && (feed.description || feed.id)) || "未命名内容";
  }

  function get_feed_cover(feed) {
    var media =
      feed && feed.objectDesc && feed.objectDesc.media && feed.objectDesc.media[0];
    if (!media) {
      return "";
    }
    return (
      media.coverUrl ||
      media.thumbUrl ||
      media.fullThumbUrl ||
      media.fullUrl ||
      media.url ||
      ""
    );
  }

  function get_feed_time(feed) {
    var timestamp = Number((feed && (feed.createtime || feed.createTime)) || 0);
    if (!timestamp) {
      return "时间未知";
    }
    if (timestamp < 1000000000000) {
      timestamp *= 1000;
    }
    var date = new Date(timestamp);
    if (Number.isNaN(date.getTime())) {
      return "时间未知";
    }
    return date.toLocaleString("zh-CN", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function create_element(tag, text) {
    var element = document.createElement(tag);
    if (text !== undefined) {
      element.textContent = text;
    }
    return element;
  }

  function get_username() {
    var { href } = window.location;
    if (!href) {
      return "";
    }
    var queries = WXU.get_queries(href);
    return queries.username || "";
  }

  function create_selector_panel(username) {
    if (selector_panel) {
      selector_panel.remove();
      selector_panel = null;
    }

    var state = {
      feeds: [],
      selected: new Set(),
      last_buffer: "",
      has_more: true,
      loading: false,
      downloading: false,
      sort: "newest",
      min_likes: "",
    };

    var overlay = create_element("div");
    overlay.style.cssText =
      "position:fixed;inset:0;z-index:2147483000;background:rgba(0,0,0,.42);display:flex;align-items:center;justify-content:center;padding:24px;box-sizing:border-box;";
    var panel = create_element("div");
    panel.style.cssText =
      "width:min(960px,96vw);height:min(760px,92vh);background:#fff;border-radius:14px;box-shadow:0 18px 60px rgba(0,0,0,.25);display:flex;flex-direction:column;overflow:hidden;color:#1f2329;font-size:14px;";
    overlay.appendChild(panel);

    var header = create_element("div");
    header.style.cssText =
      "display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid #e5e6eb;";
    var title_wrap = create_element("div");
    var title = create_element("div", "选择要下载的作品");
    title.style.cssText = "font-size:18px;font-weight:600;";
    var subtitle = create_element(
      "div",
      "直接从当前视频号主页选择，不需要先导入内容库",
    );
    subtitle.style.cssText = "margin-top:4px;color:#86909c;font-size:12px;";
    title_wrap.append(title, subtitle);
    var close_btn = create_element("button", "关闭");
    close_btn.type = "button";
    close_btn.style.cssText =
      "border:0;background:transparent;color:#4e5969;cursor:pointer;padding:6px 8px;";
    header.append(title_wrap, close_btn);
    panel.appendChild(header);

    var toolbar = create_element("div");
    toolbar.style.cssText =
      "display:flex;gap:10px;align-items:center;flex-wrap:wrap;padding:12px 20px;border-bottom:1px solid #f0f0f0;background:#fafafa;";

    var sort_select = create_element("select");
    sort_select.style.cssText =
      "height:34px;border:1px solid #d9d9d9;border-radius:6px;padding:0 28px 0 10px;background:#fff;";
    [
      ["newest", "最新发布"],
      ["oldest", "最早发布"],
      ["likes", "点赞最多"],
    ].forEach(([value, label]) => {
      var option = create_element("option", label);
      option.value = value;
      sort_select.appendChild(option);
    });

    var like_input = create_element("input");
    like_input.type = "number";
    like_input.min = "0";
    like_input.placeholder = "最低点赞数";
    like_input.style.cssText =
      "height:34px;width:130px;border:1px solid #d9d9d9;border-radius:6px;padding:0 10px;box-sizing:border-box;";

    var select_all_btn = create_element("button", "全选当前结果");
    var clear_btn = create_element("button", "清空选择");
    [select_all_btn, clear_btn].forEach((button) => {
      button.type = "button";
      button.style.cssText =
        "height:34px;border:1px solid #d9d9d9;border-radius:6px;background:#fff;padding:0 12px;cursor:pointer;";
    });

    var summary = create_element("span", "已加载 0 · 已选 0");
    summary.style.cssText = "margin-left:auto;color:#4e5969;font-size:12px;";
    toolbar.append(sort_select, like_input, select_all_btn, clear_btn, summary);
    panel.appendChild(toolbar);

    var list = create_element("div");
    list.style.cssText =
      "flex:1;overflow:auto;padding:8px 20px 16px;box-sizing:border-box;";
    panel.appendChild(list);

    var footer = create_element("div");
    footer.style.cssText =
      "display:flex;align-items:center;gap:10px;padding:12px 20px;border-top:1px solid #e5e6eb;background:#fff;";
    var status = create_element("span", "");
    status.style.cssText = "flex:1;color:#86909c;font-size:12px;";
    var load_more_btn = create_element("button", "加载更多");
    var download_btn = create_element("button", "下载选中 0");
    [load_more_btn, download_btn].forEach((button) => {
      button.type = "button";
      button.style.cssText =
        "height:36px;border-radius:7px;padding:0 16px;cursor:pointer;font-weight:500;";
    });
    load_more_btn.style.border = "1px solid #d9d9d9";
    load_more_btn.style.background = "#fff";
    download_btn.style.border = "1px solid #07c160";
    download_btn.style.background = "#07c160";
    download_btn.style.color = "#fff";
    footer.append(status, load_more_btn, download_btn);
    panel.appendChild(footer);

    function filtered_feeds() {
      var min_likes = state.min_likes === "" ? null : Number(state.min_likes);
      var result = state.feeds.filter((feed) => {
        if (min_likes === null || !Number.isFinite(min_likes)) {
          return true;
        }
        var likes = get_metrics(feed).like;
        return likes.known && likes.value >= min_likes;
      });
      result.sort((a, b) => {
        if (state.sort === "likes") {
          var left = get_metrics(a).like;
          var right = get_metrics(b).like;
          if (left.known !== right.known) {
            return left.known ? -1 : 1;
          }
          if (left.known && right.known && left.value !== right.value) {
            return right.value - left.value;
          }
        }
        var left_time = Number(a.createtime || a.createTime || 0);
        var right_time = Number(b.createtime || b.createTime || 0);
        if (state.sort === "oldest") {
          return left_time - right_time;
        }
        return right_time - left_time;
      });
      return result;
    }

    function update_summary(visible_count) {
      summary.textContent = `已加载 ${state.feeds.length} · 当前结果 ${visible_count} · 已选 ${state.selected.size}`;
      download_btn.textContent = `下载选中 ${state.selected.size}`;
      download_btn.disabled = state.selected.size === 0 || state.downloading;
      download_btn.style.opacity = download_btn.disabled ? ".55" : "1";
      load_more_btn.disabled = state.loading || !state.has_more;
      load_more_btn.textContent = state.loading
        ? "加载中..."
        : state.has_more
          ? "加载更多"
          : "已全部加载";
      load_more_btn.style.opacity = load_more_btn.disabled ? ".55" : "1";
    }

    function render() {
      var feeds = filtered_feeds();
      list.replaceChildren();
      if (feeds.length === 0) {
        var empty = create_element(
          "div",
          state.feeds.length === 0
            ? "正在读取当前账号的作品..."
            : "没有符合筛选条件的作品",
        );
        empty.style.cssText =
          "padding:48px 12px;text-align:center;color:#86909c;";
        list.appendChild(empty);
        update_summary(0);
        return;
      }

      feeds.forEach((feed) => {
        var id = get_feed_id(feed);
        if (!id) {
          return;
        }
        var metrics = get_metrics(feed);
        var row = create_element("label");
        row.style.cssText =
          "display:grid;grid-template-columns:28px 86px minmax(0,1fr) 220px;gap:12px;align-items:center;padding:12px 4px;border-bottom:1px solid #f2f3f5;cursor:pointer;";

        var checkbox = create_element("input");
        checkbox.type = "checkbox";
        checkbox.checked = state.selected.has(id);
        checkbox.style.cssText = "width:16px;height:16px;";
        checkbox.onchange = () => {
          if (checkbox.checked) {
            state.selected.add(id);
          } else {
            state.selected.delete(id);
          }
          update_summary(feeds.length);
        };

        var cover_url = get_feed_cover(feed);
        var cover;
        if (cover_url) {
          cover = create_element("img");
          cover.src = cover_url;
          cover.alt = "";
          cover.style.cssText =
            "width:86px;height:64px;border-radius:7px;object-fit:cover;background:#f2f3f5;";
        } else {
          cover = create_element("div", "无封面");
          cover.style.cssText =
            "width:86px;height:64px;border-radius:7px;background:#f2f3f5;display:flex;align-items:center;justify-content:center;color:#86909c;font-size:12px;";
        }

        var info = create_element("div");
        info.style.cssText = "min-width:0;";
        var feed_title = create_element("div", get_feed_title(feed));
        feed_title.style.cssText =
          "font-weight:500;line-height:1.45;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;";
        var feed_time = create_element("div", get_feed_time(feed));
        feed_time.style.cssText = "margin-top:6px;color:#86909c;font-size:12px;";
        info.append(feed_title, feed_time);

        var metric_box = create_element("div");
        metric_box.style.cssText =
          "display:grid;grid-template-columns:repeat(2,1fr);gap:7px 12px;color:#4e5969;font-size:12px;";
        [
          ["赞", metrics.like],
          ["评", metrics.comment],
          ["转", metrics.share],
          ["藏", metrics.collect],
        ].forEach(([label, metric]) => {
          var item = create_element("span", `${label} ${format_metric(metric)}`);
          if (!metric.known) {
            item.title = "该接口未返回此指标，不按 0 处理";
            item.style.color = "#b0b4bd";
          }
          metric_box.appendChild(item);
        });

        row.append(checkbox, cover, info, metric_box);
        list.appendChild(row);
      });
      update_summary(feeds.length);
    }

    async function load_more() {
      if (state.loading || !state.has_more) {
        return;
      }
      state.loading = true;
      status.textContent = "正在读取视频号主页数据...";
      update_summary(filtered_feeds().length);
      try {
        var payload = {
          username,
          finderUsername: my_username || username,
          lastBuffer: state.last_buffer,
          needFansCount: 0,
          objectId: "0",
        };
        var r = await WXU.API.finderUserPage(payload);
        if (!r || r.errCode !== 0) {
          throw new Error((r && r.errMsg) || "读取作品失败");
        }
        var incoming = (r.data && r.data.object) || [];
        var known_ids = new Set(state.feeds.map(get_feed_id));
        incoming.forEach((feed) => {
          var id = get_feed_id(feed);
          if (id && !known_ids.has(id)) {
            known_ids.add(id);
            state.feeds.push(feed);
          }
        });
        state.last_buffer = (r.data && r.data.lastBuffer) || "";
        state.has_more = Boolean(state.last_buffer) && incoming.length >= 15;
        status.textContent = incoming.length
          ? `本次读取 ${incoming.length} 条作品`
          : "没有更多作品";
      } catch (error) {
        state.has_more = false;
        status.textContent = error.message || "读取作品失败";
        WXU.error({
          source: "channels.profile.js:load_more",
          msg: error.message || "读取作品失败",
          alert: 0,
        });
      } finally {
        state.loading = false;
        render();
      }
    }

    sort_select.onchange = () => {
      state.sort = sort_select.value;
      render();
    };
    like_input.oninput = () => {
      state.min_likes = like_input.value;
      render();
    };
    select_all_btn.onclick = () => {
      filtered_feeds().forEach((feed) => {
        var id = get_feed_id(feed);
        if (id) {
          state.selected.add(id);
        }
      });
      render();
    };
    clear_btn.onclick = () => {
      state.selected.clear();
      render();
    };
    load_more_btn.onclick = () => {
      load_more();
    };
    close_btn.onclick = () => {
      overlay.remove();
      selector_panel = null;
    };
    overlay.onclick = (event) => {
      if (event.target === overlay) {
        close_btn.click();
      }
    };
    download_btn.onclick = async () => {
      if (state.downloading || state.selected.size === 0) {
        return;
      }
      var selected_feeds = state.feeds.filter((feed) =>
        state.selected.has(get_feed_id(feed)),
      );
      if (selected_feeds.length === 0) {
        return;
      }
      state.downloading = true;
      status.textContent = `正在创建 ${selected_feeds.length} 个下载任务...`;
      update_summary(filtered_feeds().length);
      try {
        var [err, data] = await WXU.downloader.create(selected_feeds, {
          platform: "wxchannels",
          ignore_live_feed: true,
        });
        if (err) {
          throw err;
        }
        var created = (data && data.ids && data.ids.length) || 0;
        status.textContent = `已提交 ${selected_feeds.length} 条，创建 ${created} 个下载任务`;
        WXU.downloader.show();
      } catch (error) {
        status.textContent = error.message || "创建下载任务失败";
        WXU.error({
          source: "channels.profile.js:download_selected",
          msg: error.message || "创建下载任务失败",
        });
      } finally {
        state.downloading = false;
        update_summary(filtered_feeds().length);
      }
    };

    document.body.appendChild(overlay);
    selector_panel = overlay;
    render();
    load_more();
  }

  function __wx_insert_batch_download_btn() {
    const $operation = document.querySelector(".opr-area");
    if (!$operation) {
      return false;
    }
    if ($operation.querySelector("[data-wxu-profile-selector]")) {
      return true;
    }
    const $btn = document.createElement("button");
    $btn.dataset.wxuProfileSelector = "1";
    $btn.className = "button h-7 ml-2 weui-btn weui-btn_default weui-btn_mini";
    $btn.innerText = "选择下载";
    $btn.onclick = () => {
      if (!WXU.API.finderUserPage) {
        WXU.error({
          source: "channels.profile.js:selector",
          msg: "API 未完成初始化",
        });
        return;
      }
      var username = get_username();
      if (!username) {
        WXU.error({
          source: "channels.profile.js:selector",
          msg: "username 不能为空",
        });
        return;
      }
      create_selector_panel(username);
    };
    $operation.appendChild($btn);
    return true;
  }

  WXU.onInit((data) => {
    my_username = data.mainFinderUsername;
  });
  WXU.observe_node({
    selector: ".opr-area",
    container: "#app",
    onOk: () => {
      __wx_insert_batch_download_btn();
    },
  });
})();
