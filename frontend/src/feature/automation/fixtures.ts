import type { Account, Post, TestComment } from "./model/types";

export const accounts: Account[] = [
  {
    id: "instagram",
    name: "Instagram",
    handle: "@northstar.studio",
    platform: "instagram",
    canSendDM: true,
  },
  {
    id: "facebook",
    name: "Facebook",
    handle: "Northstar Studio",
    platform: "facebook",
    canSendDM: true,
  },
  {
    id: "youtube",
    name: "YouTube",
    handle: "@northstarstudio",
    platform: "youtube",
    canSendDM: false,
  },
];

export const posts: Post[] = [
  {
    id: "ig_1",
    accountId: "instagram",
    title: "A little idea. A big beginning.",
    date: "Oct 30",
    cover: "rose",
    copy: "SMALL IDEAS.\nBIG ENERGY.",
    comments: 128,
  },
  {
    id: "ig_2",
    accountId: "instagram",
    title: "Your next chapter starts here.",
    date: "Oct 28",
    cover: "violet",
    copy: "MAKE\nSOMETHING\nMATTER.",
    comments: 76,
  },
  {
    id: "fb_1",
    accountId: "facebook",
    title: "Meet your new creative toolkit.",
    date: "Oct 29",
    cover: "orange",
    copy: "LESS BUSY.\nMORE YOU.",
    comments: 54,
  },
  {
    id: "yt_1",
    accountId: "youtube",
    title: "A better way to create.",
    date: "Oct 27",
    cover: "blue",
    copy: "CREATE\nYOUR\nNEXT.",
    comments: 93,
  },
];

export const initialComment: TestComment = {
  accountId: "instagram",
  postId: "ig_1",
  text: "Hey! Can you send me your pricing? ✨",
  isOwn: false,
  activeConversation: false,
  previouslyStarted: false,
};
